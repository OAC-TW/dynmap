package webmap

import (
	"encoding/json"
	"net/http"
	"strconv"
	//"path"
	//"path/filepath"
)

func (wb *WebAPI) config(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
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

	switch r.Method {
	case "GET": // get config
		list := wb.db.GetConfig()
		enc := json.NewEncoder(w)
		err := enc.Encode(list)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}


	case "POST": // set config
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		conf := wb.db.GetConfig().Clone()
		conf.VersionA = genVersion()

		act := getKey(r.URL.Path)
		switch act {
		case "": // all
			conf.SiteTitle = r.Form.Get("title")
			//conf.Logo = filepath.FromSlash(path.Clean("/"+r.Form.Get("logo")))
			conf.Logo = r.Form.Get("logo")
			conf.Lang = r.Form.Get("lang")
			conf.Manifest = r.Form.Get("manifest")
			conf.HtmlHead = r.Form.Get("head")

			val, err := strconv.ParseInt(r.Form.Get("loadfs"), 10, 64)
			if err == nil {
				if val < 0 {
					val = 0
				}
				conf.LoadLimit = int64(val)
			}

			conf.CountStats = false
			if r.Form.Get("cstats") == "1" {
				conf.CountStats = true
			}
			conf.ShowStats = false
			if r.Form.Get("stats") == "1" {
				conf.ShowStats = true
			}
			conf.ShowLink = false
			if r.Form.Get("link") == "1" {
				conf.ShowLink = true
			}
			conf.ShowUserLoad = false
			if r.Form.Get("load") == "1" {
				conf.ShowUserLoad = true
			}

		default:
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		wb.db.SetConfig(conf) // write back
		wb.updateTmpl() // re-parse when VersionA update

		writeResp(w, true, "")
	}
}


