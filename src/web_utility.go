package webmap

import (
	//"crypto/sha256"
 	//"crypto/rand"
	//"encoding/base64"
	//"encoding/binary"
	//"encoding/hex"
	"fmt"
	//"io"
	"time"
	"net/http"
	"path/filepath"
	"strings"
	"strconv"
	"log"
)

var (
	_SESSION_COOKIE = "map-session"
	_COOKIE_TTL = _SESSION_TTL

	_USER_VISIT_COOKIE = "uv-temp"
	_USER_VISIT_TTL = 20 * 60 * time.Second // 20 min
)

type SessHandlerFunc func(base string, sd *SessionData, w http.ResponseWriter, r *http.Request)

func setCookie(w http.ResponseWriter, token string) {
	// update cookie
	cookie := &http.Cookie{
		Name: _SESSION_COOKIE,
		Value: token,
		Path: "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires: now().Add(_COOKIE_TTL),
		MaxAge: int(_COOKIE_TTL.Seconds()),
	}
	http.SetCookie(w, cookie)
}

func getSess(sess *Session, w http.ResponseWriter, r *http.Request) *SessionData {
	cookie, err := r.Cookie(_SESSION_COOKIE)
	if err != nil || cookie.Value == "" {
		return nil
	}

	token := cookie.Value
	sd := sess.GetOrRenewSession(token)
	if sd == nil { // timeout?
		return nil
	}

	// update cookie
	setCookie(w, token)

	return sd
}

func delSess(sess *Session, w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(_SESSION_COOKIE)
	if err != nil || cookie.Value == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	token := cookie.Value
	sess.Destroy(token)

	// force cookie timeout
	cookieSet := &http.Cookie{
		Name: _SESSION_COOKIE,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires: now(),
		MaxAge: -1,
	}
	http.SetCookie(w, cookieSet)
}

func startSess(sess *Session, w http.ResponseWriter, r *http.Request) *SessionData {
	token, sd := sess.NewToken()
	if sd == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil
	}

	// update cookie
	setCookie(w, token)
	return sd
}

func addUV(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(_USER_VISIT_COOKIE)
	if err == nil {
		return false
	}

	cookie = &http.Cookie{
		Name: _USER_VISIT_COOKIE,
		Value: "UserVisitFlag",
		Path: "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires: now().Add(_USER_VISIT_TTL),
		MaxAge: int(_USER_VISIT_TTL.Seconds()),
	}
	http.SetCookie(w, cookie)
	return true
}

func writeResp(w http.ResponseWriter, ok bool, errMsg string) {
	if ok {
		w.Write([]byte(`{"ok": true}`))
		return
	}

	str := fmt.Sprintf(`{"ok": %v, "msg": "%v"}`, ok, errMsg)
	w.Write([]byte(str))
}

func logRequest(w http.ResponseWriter, r *http.Request) {
	Vln(2, "[web][req]", r.RemoteAddr, r.Method, r.URL, r.Referer(), r.UserAgent(), r.Header.Get("X-Forwarded-For"))
}

func ReqLogFn(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lrw := NewlogResponseWriter(w)
		f(lrw, r)

		//$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent" "$http_x_forwarded_for"
		statusCode := lrw.statusCode
		bytesSent := lrw.bytesSent
		LogWebf("%v \"%v %v %v\" %d %d \"%v\" \"%v\" \"%v\"\n", r.RemoteAddr, r.Method, r.URL, r.Proto, statusCode, bytesSent, r.Referer(), r.UserAgent(), r.Header["X-Forwarded-For"])
		//logRequest(w, r)
	}
}

func ReqLog(next http.Handler) http.Handler {
	return http.HandlerFunc(ReqLogFn(next.ServeHTTP))
}

type logResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytesSent int
}

func NewlogResponseWriter(w http.ResponseWriter) *logResponseWriter {
	return &logResponseWriter{w, http.StatusOK, 0}
}

func (lrw *logResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *logResponseWriter) Write(buf []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(buf)
	lrw.bytesSent += n
	return n, err
}


func ReqGzFn(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gzw, ok := TryGzipResponse(w, r)
		if !ok {
			next(w, r)
			return
		}
		defer gzw.Close()
		next(gzw, r)
	}
}

func ReqGz(next http.Handler) http.Handler {
	return http.HandlerFunc(ReqGzFn(next.ServeHTTP))
}

// "public, no-cache, max-age=0, must-revalidate", cache but check 304 every time
// "public, max-age=31536000, immutable", static file for same url
func ReqCacheFn(next http.HandlerFunc, cacheControl string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl)
		next(w, r)
	}
}

func ReqCache(next http.Handler, cacheControl string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl)
		next.ServeHTTP(w, r)
	})
}

func reqAGP(base string, sess *Session, f SessHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "HEAD":
			return
		case "OPTIONS":
			w.Header().Add("Allow", "GET, HEAD, POST, OPTIONS")
			return
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return

		case "GET":
		case "POST":
		}
		sd := getSess(sess, w, r)
		if sd == nil { // 403
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		f(base, sd, w, r)
	}
}

func reqAG(base string, sess *Session, f SessHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		sd := getSess(sess, w, r)
		if sd == nil { // 403
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		f(base, sd, w, r)
	}
}

func reqAP(base string, sess *Session, f SessHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "HEAD":
			return
		case "OPTIONS":
			w.Header().Add("Allow", "HEAD, POST, OPTIONS")
			return
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		case "POST":
		}
		sd := getSess(sess, w, r)
		if sd == nil { // 403
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		f(base, sd, w, r)
	}
}

func reqG(base string, sess *Session, f SessHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		sd := getSess(sess, w, r)
		f(base, sd, w, r)
	}
}

func reqP(base string, sess *Session, f SessHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "HEAD":
			return
		case "OPTIONS":
			w.Header().Add("Allow", "POST, HEAD, OPTIONS")
			return
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return

		case "POST":
		}
		sd := getSess(sess, w, r)
		f(base, sd, w, r)
	}
}

func getKey(url string) string {
	_, b := filepath.Split(url)
	return b
}

func getParm(url string, prefix string) (string, string) {
	v := strings.SplitN(url, prefix, 2)
	if len(v) < 2 {
		return "", ""
	}
	a := v[1]
	b := ""
	for i, r := range v[1] {
		if r == '/' {
			a = v[1][:i]
			b = v[1][i+1:]
		}
	}
	return a, b
}

func splitParms(parm string) []uint64 {
	const MAX = 500
	if len(parm) > (20*MAX) {
		return nil
	}
	strs := strings.SplitN(parm, ",", MAX)
	ret := make([]uint64, 0, len(strs))
	for _, v := range strs {
		id, err := strconv.ParseUint(v, 10, 64)
		if err == nil {
			ret = append(ret, id)
		}
	}
	return ret
}

func getIP(remoteAddr string) string {
	v := strings.SplitN(remoteAddr, ":", 2)
	return v[0]
}


// for web handler use
var weblogger *Logger // stderr
func SetWebOutput(filePath string) error {
	var err error
	weblogger, err = NewLoggerFile(nil, filePath, log.LstdFlags|log.Lmicroseconds)
	return err
}
func LogWebf(format string, v ...interface{}) {
	if weblogger != nil {
		weblogger.Printf(format, v...)
	}
}

