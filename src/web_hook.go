package webmap

import (
	"encoding/json"
	"strconv"
	"net/http"
	"os"
	"path/filepath"
)


func (wb *WebAPI) hookDL(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
	fileToken := filepath.Base(r.URL.Path) // TODO: check 'asd?foo=bar'
	hook := wb.db.GetHookByToken(fileToken)
	if hook == nil {
		goto ERR404
	}

	if hook.Disable { // deleted
		if sd == nil {
			goto ERR404
		}
		uid, ok := sd.Get("acc")
		if !ok {
			goto ERR404
		}
		u := wb.db.GetUserByUID(uid.(UserID))
		if u == nil {
			goto ERR404
		}
		if u.Freeze {
			goto ERR404
		}
		goto DL
	}

DL:
	hook.ServeContent(w, r, CacheFileDir)
	return

ERR404:
	http.Error(w, "404 not found", http.StatusNotFound)
	return
}

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
				hook := wb.db.GetHookByID(HookID(id))
				if hook == nil {
					http.Error(w, "404 not found", http.StatusNotFound)
					return
				}

				wb.db.DelHookByID(HookID(id))
				err = hook.DelFromFS(CacheFileDir)
				if err != nil {
					Vln(3, "[web][hook]remove cached data error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				}

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

// update hook data
func (wb *WebAPI) hookUpdate(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
	authToken := filepath.Base(r.URL.Path) // TODO: check 'asd?foo=bar'
	hook0 := wb.db.GetHookByAuthToken(authToken)
	if hook0 == nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}

	r.ParseMultipartForm(CacheFileSizeLimit)

	file, handler, err := r.FormFile("file")

	// TODO: check meta
	// check file magic
	if isExeFile(file) {
		Vln(3, "[web][hook]try upload executable file!", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), handler.Filename, handler.Size, handler.Header)
		http.Error(w, "file type not allow", http.StatusForbidden)
		return
	}
	hook := hook0.Clone()
	hook.SetData(handler.Filename, handler.Size)

	saveFp := filepath.Join(CacheFileDir, hook.SaveName)
	f, err := os.OpenFile(saveFp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		Vln(3, "[web][hook]open save file error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// TODO: also save in-memory cache if size < CacheInMemorySizeLimit
	hash, err := cpAndHashFd(f, file)
	if err != nil {
		Vln(3, "[web][hook]save and hash file error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	hook.Checksum = hash

	err = wb.db.UpdateHook(hook)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		Vln(3, "[web][hook]open save file error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
	}

	// remove old data
	err = hook0.DelFromFS(CacheFileDir)
	if err != nil {
		Vln(3, "[web][hook]remove old cached data error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
	}
	return
}

