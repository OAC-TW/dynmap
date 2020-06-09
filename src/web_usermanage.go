package webmap

import (
	"encoding/json"
	"strconv"
	"net/http"
)


// list / new / edit / freeze user
func (wb *WebAPI) usermanage(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
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
	if !u.Super {
		http.Error(w, "Forbidden, no permission", http.StatusForbidden)
		return
	}

	parseUser := func() (*User, string) {
		u := &User{
			Acc: r.Form.Get("acc"),

			Name: r.Form.Get("name"),
			Note: r.Form.Get("note"),
		}

		u.Freeze = false
		if r.Form.Get("fz") == "1" {
			u.Freeze = true
		}

		u.Super = false
		if r.Form.Get("su") == "1" {
			u.Super = true
		}

		pwd := r.Form.Get("pwd")
		if pwd != "" {
			if len(pwd) < 3 {
				return nil, "password length too short"
			}
			if len(pwd) > 80 {
				return nil, "password length too long"
			}
			u.SetPasswd(pwd)
		}
		return u, ""
	}


	switch r.Method {
	case "GET":
		uid := getKey(r.URL.Path)
		if uid != "" { // get user
			id, err := strconv.ParseUint(uid, 10, 64)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			u := wb.db.GetUserByUID(UserID(id))
			if u == nil {
				http.Error(w, "404 not found", http.StatusNotFound)
				return
			}
			u = u.Clone()
			u.Hash = ""

			enc := json.NewEncoder(w)
			err = enc.Encode(u)
			if err != nil {
				// should not error, log it
				Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
			}
			return
		}

		// get all users
		list := wb.db.ListUser()
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}

	case "POST": // new / update user
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		uid, act := getParm(r.URL.Path, base)
		switch uid {
		case "": // new
			usr, msg := parseUser()
			if msg != "" {
				writeResp(w, false, msg)
				return
			}
			_, err = wb.db.AddUser(usr)
			if err != nil {
				if err == ErrAccExist { // if error by acc exist
					writeResp(w, false, "login account exist")
					return
				}
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
				err = wb.db.DelUserByUID(UserID(id))

			default:
				usr, msg := parseUser()
				if msg != "" {
					writeResp(w, false, msg)
					return
				}
				usr.ID = UserID(id)
				err = wb.db.UpdateUser(usr)
			}
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			// http return ok
			Vln(3, "[web][user]updated", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
			writeResp(w, true, "")
			return
		}
	}

}

