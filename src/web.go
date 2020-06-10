package webmap

import (
	"bytes"
	//"crypto/sha256"
 	//"crypto/rand"
	//"encoding/base64"
	//"encoding/binary"
	//"encoding/hex"
	"encoding/json"
	//"strconv"
	//"fmt"
	//"io"
	//"os"
	"sync/atomic"
	"time"
	"net/http"
	"html/template"
	txtemplate "text/template"
	//"path/filepath"
)

var (
	_LOGIN_DELAY = 500 * time.Millisecond
	_IP_BAN_COUNT = 20
	_ACC_BAN_COUNT = 5
)

type WebAPI struct {
	*http.ServeMux
	db API
	sess *Session
	f2b *Fail2Ban

	indexBuf atomic.Value // *WebCacheResp
	swBuf atomic.Value // *WebCacheResp
	mfBuf atomic.Value // *WebCacheResp
}

func NewWebAPI(api API) *WebAPI { // http.Handler
	web := &WebAPI{
		ServeMux: http.NewServeMux(),
		db: api,
		sess: NewSession(),
		f2b: NewFail2Ban(),
	}
	web.initHandler()
	web.updateTmpl()

	return web
}

func (wb *WebAPI) initHandler() {
	// public api
	wb.HandleFunc("/", wb.index)
	wb.HandleFunc("/api/info", wb.info)
	wb.HandleFunc("/api/stats", wb.stats)
	wb.HandleFunc("/sw.js", ReqGzFn(wb.swjs))
	wb.HandleFunc("/manifest.json", ReqGzFn(wb.manifest))
	wb.HandleFunc("/dl/", ReqCacheFn(ReqGzFn(reqG("/dl/", wb.sess, wb.download)), "public, max-age=31536000, immutable"))

	wb.HandleFunc("/hook/", ReqCacheFn(ReqGzFn(reqG("/hook/", wb.sess, wb.hookDL)), "public, no-cache, max-age=0, must-revalidate"))
	wb.HandleFunc("/api/push/", reqP("/api/push/", wb.sess, wb.hookUpdate)) // data input

	// user
	wb.HandleFunc("/api/auth", wb.auth)
	wb.HandleFunc("/api/login", wb.logIn)
	wb.HandleFunc("/api/logout", wb.logOut)
	wb.HandleFunc("/api/user", reqAGP("/api/user", wb.sess, wb.user))

	// mgr
	wb.HandleFunc("/api/usermanage/", reqAGP("/api/usermanage/", wb.sess, wb.usermanage))
	wb.HandleFunc("/api/layer/", reqAGP("/api/layer/", wb.sess, wb.layer))
	wb.HandleFunc("/api/map/", reqAGP("/api/map/", wb.sess, wb.basemap)) // basemap
	wb.HandleFunc("/api/tab/", reqAGP("/api/tab/", wb.sess, wb.tab)) // tab
	wb.HandleFunc("/api/link/", reqAGP("/api/link/", wb.sess, wb.link)) // nesed links
	wb.HandleFunc("/api/attach/", reqAGP("/api/attach/", wb.sess, wb.attach)) // attach
	wb.HandleFunc("/api/hook/", reqAGP("/api/hook/", wb.sess, wb.hook)) // hook

	wb.HandleFunc("/api/config/", reqAGP("/api/config/", wb.sess, wb.config)) // config
	wb.HandleFunc("/api/astats", reqAG("/api/astats", wb.sess, wb.astats))
}

func (wb *WebAPI) logIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t0 := time.Now().Add(_LOGIN_DELAY)
	ip := getIP(r.RemoteAddr)

	if wb.f2b.IsBanIP(ip, _IP_BAN_COUNT) { // banned IP
		Vln(3, "[web][login][IP banned by system]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])
		time.Sleep(t0.Sub(time.Now())) // block untill time up
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// login part
	user := r.Form.Get("acc")
	pwd := r.Form.Get("pwd")

	if wb.f2b.IsBanAcc(user, _ACC_BAN_COUNT) { // banned acc
		Vln(3, "[web][login][Acc banned by system]", user, r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])
		time.Sleep(t0.Sub(time.Now())) // block untill time up
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	u := wb.db.GetUserByAcc(user)
	if u == nil {
		wb.f2b.AddFail(ip, user) // add to filter
		Vln(3, "[web][login][non-existing account]", user, r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])

		time.Sleep(t0.Sub(time.Now())) // block untill time up
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if u.Freeze {
		wb.f2b.AddFail(ip, user) // add to filter
		Vln(3, "[web][login][freeze account]", user, r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])

		time.Sleep(t0.Sub(time.Now())) // block untill time up
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	ok := u.CheckPasswd(pwd)
	if !ok {
		wb.f2b.AddFail(ip, user) // add to filter
		Vln(3, "[web][login][auth failed]", user, r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])

		time.Sleep(t0.Sub(time.Now())) // block untill time up
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	sd := startSess(wb.sess, w, r)
	if sd == nil {
		time.Sleep(t0.Sub(time.Now())) // block untill time up
		return
	}
	sd.Set("acc", u.ID)

	Vln(3, "[web][login][success]", user, r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])
	time.Sleep(t0.Sub(time.Now())) // block untill time up

	// http return ok
	writeResp(w, true, "")
}

func (wb *WebAPI) logOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	delSess(wb.sess, w, r)

	Vln(3, "[web][logout]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])
	// http return ok
	writeResp(w, true, "")
}

func (wb *WebAPI) user(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
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
	case "GET": // get self info
		u = u.Clone()
		u.Hash = "" // remove password hash

		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate") // has user info, not cache by other
		enc := json.NewEncoder(w)
		err := enc.Encode(u)
		if err != nil {
			// should not error, log it
			Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
		}

	case "POST": // update password
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		name := r.Form.Get("name")

		pwd := r.Form.Get("pwd")
		pwd2 := r.Form.Get("pwd2") // new password

		if name == "" {
			writeResp(w, false, "empty name") // return failed
			return
		}

		ok = u.CheckPasswd(pwd)
		if !ok { // old password not match
			writeResp(w, false, "old password wrong") // return failed
			return
		}

		if pwd2 != "" {
			// TODO: check password complexity
			err = u.SetPasswd(pwd2)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				Vln(2, "[web][err]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
				return
			}
		}

		u.Name = name

		err = wb.db.UpdateUser(u)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			Vln(2, "[web][err]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
			return
		}

		// http return ok
		Vln(3, "[web][user]updated password", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
		writeResp(w, true, "")
	}
}

func (wb *WebAPI) auth(w http.ResponseWriter, r *http.Request) {
	sd := getSess(wb.sess, w, r)
	if sd == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	uid, ok := sd.Get("acc")
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	u := wb.db.GetUserByUID(uid.(UserID))
	if u == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if u.Freeze {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	type Info struct {
		User  []*User       `json:"user,omitempty"`
	}
	out := &Info{}

	if u.Super { // only super user can list all user
		users := wb.db.ListUser()
		if users != nil {
			out.User = users
		}
	} else {
		u = u.Clone()
		u.Hash = "" // remove password hash
		out.User = []*User{u}
	}

	w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate") // for login (has user info)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(out)
	if err != nil {
		// should not error, log it
		Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
	}
}

func (wb *WebAPI) info(w http.ResponseWriter, r *http.Request) {
	// TODO: Etag & Cache-Control
	type Info struct {
		Layer []*LayerGroup `json:"layer"`
		Map   []*BaseMap    `json:"map"`
		Tabs  []*TabData    `json:"tab,omitempty"`
		Link  []*Link       `json:"link,omitempty"`
		User  []*User       `json:"user,omitempty"`
	}

	out := &Info{}

	layer := wb.db.GetPubLayer()
	if layer != nil {
		out.Layer = layer
	}

	maps := wb.db.GetPubMap()
	if maps != nil {
		out.Map = maps
	}

	links := wb.db.GetPubLink()
	if links != nil {
		out.Link = links
	}

	tabs := wb.db.GetPubTab()
	if tabs != nil {
		out.Tabs = tabs
	}

	w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate") // for non-login
	sd := getSess(wb.sess, w, r)
	if sd != nil {
		uid, ok := sd.Get("acc")
		if !ok {
			goto Flush
		}
		u := wb.db.GetUserByUID(uid.(UserID))
		if u == nil {
			goto Flush
		}
		if u.Freeze {
			goto Flush
		}
		if u.Super { // only super user can list all user
			users := wb.db.ListUser()
			if users != nil {
				out.User = users
			}
		} else {
			u = u.Clone()
			u.Hash = "" // remove password hash
			out.User = []*User{u}
		}
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate") // for login (has user info)
	}


Flush:

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(out)
	if err != nil {
		// should not error, log it
		Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
	}
}

func (wb *WebAPI) stats(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "HEAD":
		return
	case "OPTIONS":
		w.Header().Add("Allow", "GET, HEAD, OPTIONS")
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return

	case "GET":
	}

	type Stats struct {
		PageView uint64 `json:"pv"`
		UserVisit uint64 `json:"uv,omitempty"`
	}

	out := &Stats{}

	sd := getSess(wb.sess, w, r)
	if sd != nil {
		uid, ok := sd.Get("acc")
		if !ok {
			goto AddAndFlush
		}
		u := wb.db.GetUserByUID(uid.(UserID))
		if u == nil {
			goto AddAndFlush
		}
		if u.Freeze {
			goto AddAndFlush
		}
		out.PageView = wb.db.GetPageView()
		out.UserVisit = wb.db.GetUserVisit()
		goto Flush
	}

AddAndFlush:
	out.PageView = wb.db.AddAndGetPageView()
	if addUV(w, r) {
		out.UserVisit = wb.db.AddAndGetUserVisit()
	} else {
		out.UserVisit = wb.db.GetUserVisit()
	}

Flush:
	w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate")
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(out)
	if err != nil {
		// should not error, log it
		Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
	}
}

func (wb *WebAPI) astats(base string, sd *SessionData, w http.ResponseWriter, r *http.Request) {
	uid, ok := sd.Get("acc")
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	u := wb.db.GetUserByUID(uid.(UserID))
	if u == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if u.Freeze {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	type Stats struct {
		PageView uint64 `json:"pv"`
		UserVisit uint64 `json:"uv,omitempty"`
	}

	out := &Stats{}
	out.PageView = wb.db.GetPageView()
	out.UserVisit = wb.db.GetUserVisit()
	w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate")
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(out)
	if err != nil {
		// should not error, log it
		Vln(2, "[web][panic]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), err)
	}
}


func (wb *WebAPI) updateTmpl() error {
	conf := wb.db.GetConfig()

	{ // index file
		t, err := template.New("index").Delims("[[", "]]").Funcs(template.FuncMap{
			"htmlSafe": func(html string) template.HTML {
				return template.HTML(html)
			},
		}).ParseFiles("index.tmpl")
		if err != nil {
			Vln(2, "[tmpl]parse", "index.tmpl", err)
			return err
		}

		var b bytes.Buffer
		err = t.ExecuteTemplate(&b, "index.tmpl", conf)
		if err != nil {
			Vln(2, "[tmpl]exec", "index.tmpl", err)
			return err
		}
		wb.indexBuf.Store(NewCache(b.Bytes()))
	}


	{ // service worker
		swT, err := txtemplate.New("index").Delims("[[", "]]").ParseFiles("sw.js.tmpl")
		if err != nil {
			Vln(2, "[tmpl]parse", "sw.js.tmpl", err)
			return err
		}

		var b bytes.Buffer
		err = swT.ExecuteTemplate(&b, "sw.js.tmpl", conf)
		if err != nil {
			Vln(2, "[tmpl]exec", "sw.js.tmpl", err)
			return err
		}
		swCache := NewCache(b.Bytes())
		wb.swBuf.Store(swCache)
	}

	// manifest.json
	wb.mfBuf.Store(NewCache([]byte(conf.Manifest)))

	return nil
}

func (wb *WebAPI) index(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "HEAD":
		return
	case "OPTIONS":
		w.Header().Add("Allow", "GET, OPTIONS")
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return

	case "GET":
	}

	switch r.URL.Path {
	case "/", "/index", "/index.html":
	default:
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	c, ok := wb.indexBuf.Load().(*WebCacheResp)
	if !ok {
		Vln(2, "[index][tmpl]load error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

	w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate")
	if !c.WriteCache(w, r, "index.html") {
		Vln(2, "[index][tmpl]cache error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (wb *WebAPI) swjs(w http.ResponseWriter, r *http.Request) {
	c, ok := wb.swBuf.Load().(*WebCacheResp)
	if !ok {
		Vln(2, "[sw][tmpl]load error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

	w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate")
	if !c.WriteCache(w, r, "sw.js") {
		Vln(2, "[sw][tmpl]cache error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (wb *WebAPI) manifest(w http.ResponseWriter, r *http.Request) {
	c, ok := wb.mfBuf.Load().(*WebCacheResp)
	if !ok {
		Vln(2, "[manifest][tmpl]load error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

	w.Header().Set("Cache-Control", "public, no-cache, max-age=0, must-revalidate")
	if !c.WriteCache(w, r, "manifest.json") {
		Vln(2, "[manifest][tmpl]cache error", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

