package webmap

import (
	"encoding/json"
	"strconv"
	"net/http"
)


// list / new / edit / del hook
func (wb *WebAPI) hook(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
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

	parseHook := func() (*HookConfig, string) {
		o := &HookConfig{
			Name: r.Form.Get("name"),
			Note: r.Form.Get("note"),

			RenderType: r.Form.Get("type"),
		}

		o.Disable = false
		if r.Form.Get("disable") == "1" {
			o.Disable = true
		}

		return o, ""
	}


	switch r.Method {
	case "GET":
		uid := getKey(r.URL.Path)
		if uid != "" { // get hook
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			hook := wb.db.GetHookByID(HookID(id))
			if hook == nil {
				http.Error(w, "404 not found", http.StatusNotFound)
				return
			}

			enc := json.NewEncoder(w)
			err = enc.Encode(hook)
			if err != nil {
				// should not error, log it
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
			}
			return
		}

		// get all hook
		list := wb.db.ListHook()
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}

	case "POST": // new / update hook
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uid, act := getParm(r.URL.Path, base)
		switch uid {
		case "": // new
			hook, msg := parseHook()
			if msg != "" {
				writeResp(w, false, msg)
				return
			}
			_, err = wb.db.AddHook(hook)
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
				err = wb.db.DelHookByID(HookID(id))

			default:
				hook, msg := parseHook()
				if msg != "" {
					writeResp(w, false, msg)
					return
				}
				hook.ID = HookID(id)
				err = wb.db.UpdateHook(hook)
			}
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			// http return ok
			Vln(3, "[web][hook]updated", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
			writeResp(w, true, "")
			return
		}
	}

}

