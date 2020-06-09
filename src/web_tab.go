package webmap

import (
	"encoding/json"
	"strconv"
	"net/http"
)


// list / new / edit / del tab
func (wb *WebAPI) tab(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
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

	parseTab := func() (*TabData, string) {
		o := &TabData{
			Title: r.Form.Get("title"),
			Note: r.Form.Get("note"),
			Icon: r.Form.Get("icon"),
			CloseIcon: r.Form.Get("cicon"),
			Data: r.Form.Get("data"),
		}

		o.Show = false
		if r.Form.Get("show") == "1" {
			o.Show = true
		}

		if o.Title == "" && o.Icon == "" {
			return nil, "need title or icon"
		}

		return o, ""
	}


	switch r.Method {
	case "GET":
		uid := getKey(r.URL.Path)
		if uid != "" { // get tab
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			tab := wb.db.GetTabByID(TabID(id))
			if tab == nil {
				http.Error(w, "404 not found", http.StatusNotFound)
				return
			}

			enc := json.NewEncoder(w)
			err = enc.Encode(tab)
			if err != nil {
				// should not error, log it
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
			}
			return
		}

		// get all tab
		list := wb.db.GetAllTab()
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}

	case "POST": // new / update tab
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uid, act := getParm(r.URL.Path, base)
		switch uid {
		case "": // new
			tab, msg := parseTab()
			if msg != "" {
				writeResp(w, false, msg)
				return
			}
			_, err = wb.db.AddTab(tab)
			if err != nil {
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			writeResp(w, true, "")
			return

		case "order": // order
			list := ([]TabID)(splitParms(r.Form.Get("order")))
			if len(list) == 0 {
				writeResp(w, false, "no valid values")
				return
			}
			err := wb.db.OrderTab(list)
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
				err = wb.db.DelTabByID(TabID(id))

			default:
				tab, msg := parseTab()
				if msg != "" {
					writeResp(w, false, msg)
					return
				}
				tab.ID = TabID(id)
				err = wb.db.UpdateTab(tab)
			}
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			// http return ok
			Vln(3, "[web][tab]updated", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
			writeResp(w, true, "")
			return
		}
	}

}

