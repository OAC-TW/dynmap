package webmap

import (
	"encoding/json"
	"strconv"
	"strings"
	"net/http"
)


// list / new / edit / del link
func (wb *WebAPI) link(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
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

	parseLink := func() (*Link, string) {
		o := &Link{
			Url: r.Form.Get("url"),
			Title: r.Form.Get("title"),

			Name: r.Form.Get("name"),
			Note: r.Form.Get("note"),
		}

		o.Hide = false
		if r.Form.Get("hide") == "1" {
			o.Hide = true
		}

		return o, ""
	}

	parseLinkOrder := func (parm string) []*LinkOrder_S {
		const EACH = 20*2 + 2
		const MAX = 500
		if len(parm) > 500*EACH {
			return nil
		}

		pairs := strings.SplitN(parm, ",", MAX)
		ret := make([]*LinkOrder_S, 0, len(pairs))
		for _, pair := range pairs {
			v := strings.SplitN(pair, "/", 2)
			if len(v) != 2 {
				continue
			}
			id, err := strconv.ParseUint(v[0], 10, 64)
			if err != nil {
				continue
			}
			lv, err := strconv.ParseUint(v[1], 10, 64)
			if err != nil {
				continue
			}
			ret = append(ret, LinkOrder(id, int(lv)))
		}
		return ret
	}

	switch r.Method {
	case "GET":
		uid := getKey(r.URL.Path)
		if uid != "" { // get link
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			bmap := wb.db.GetLinkByID(LinkID(id))
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

		// get all link
		list := wb.db.GetAllLink()
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}

	case "POST": // new / update link
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uid, act := getParm(r.URL.Path, base)
		switch uid {
		case "": // new
			lk, msg := parseLink()
			if msg != "" {
				writeResp(w, false, msg)
				return
			}
			_, err = wb.db.AddLink(lk)
			if err != nil {
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			writeResp(w, true, "")
			return

		case "order": // order
			list := parseLinkOrder(r.Form.Get("order"))
			if len(list) == 0 {
				writeResp(w, false, "no valid values")
				return
			}
			err := wb.db.OrderLink(list)
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
				err = wb.db.DelLinkByID(LinkID(id))

			default:
				lk, msg := parseLink()
				if msg != "" {
					writeResp(w, false, msg)
					return
				}
				lk.ID = LinkID(id)
				err = wb.db.UpdateLink(lk)
			}
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			// http return ok
			Vln(3, "[web][link]updated", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
			writeResp(w, true, "")
			return
		}
	}

}

