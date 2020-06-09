package webmap

import (
	"encoding/json"
	"strconv"
	"net/http"
)


// list / new / edit / del basemap
func (wb *WebAPI) basemap(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
	uid, ok := sd.Get("acc")
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	u := wb.db.GetUserByUID(uid.(UserID))
	if u == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if u.Freeze {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	parseZoom := func(input string) (int, bool) {
		if input == "" {
			return 18, false
		}
		zoom, err := strconv.ParseUint(input, 10, 8)
		if err != nil {
			return 0, false
		}
		if zoom > 18 {
			return 18, true
		}
		if zoom < 0 {
			return 0, true
		}
		return int(zoom), true
	}

	parseMap := func() (*BaseMap, string) {
		o := &BaseMap{
			Url: r.Form.Get("url"),
			SubDomain: r.Form.Get("subdomains"),
			ErrTile: r.Form.Get("errorTileUrl"),
			Attribution: r.Form.Get("attr"),

			Name: r.Form.Get("name"),
			Note: r.Form.Get("note"),
		}

		o.Hide = false
		if r.Form.Get("hide") == "1" {
			o.Hide = true
		}

		o.MaxZoom = 18
		zoom, ok := parseZoom(r.Form.Get("maxZoom"))
		if ok {
			o.MaxZoom = zoom
		}

		return o, ""
	}


	switch r.Method {
	case "GET":
		uid := getKey(r.URL.Path)
		if uid != "" { // get basemap
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			bmap := wb.db.GetMapByID(MapID(id))
			if bmap == nil {
				http.Error(w, "404 not found", http.StatusNotFound)
				return
			}

			enc := json.NewEncoder(w)
			err = enc.Encode(bmap)
			if err != nil {
				// should not error, log it
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
			}
			return
		}

		// get all basemap
		list := wb.db.GetAllMap()
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}

	case "POST": // new / update basemap
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uid, act := getParm(r.URL.Path, base)
		switch uid {
		case "": // new
			bmap, msg := parseMap()
			if msg != "" {
				writeResp(w, false, msg)
				return
			}
			_, err = wb.db.AddMap(bmap)
			if err != nil {
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			writeResp(w, true, "")
			return

		case "order": // order
			list := ([]MapID)(splitParms(r.Form.Get("order")))
			if len(list) == 0 {
				writeResp(w, false, "no valid values")
				return
			}
			err := wb.db.OrderMap(list)
			if err != nil {
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			writeResp(w, true, "")
			return

		default: // update / del
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			switch act {
			case "del":
				err = wb.db.DelMapByID(MapID(id))

			default:
				bmap, msg := parseMap()
				if msg != "" {
					writeResp(w, false, msg)
					return
				}
				bmap.ID = MapID(id)
				err = wb.db.UpdateMap(bmap)
			}
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			// http return ok
			Vln(3, "[web][basemap]updated", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
			writeResp(w, true, "")
			return
		}
	}

}

