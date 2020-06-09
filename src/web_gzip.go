package webmap

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// TODO: cache pool for gzipped resp
var (
	GZIP_LV = 5
	//GZIP_POOL = 32 * 1024 * 1024 // 32MB
)

type GzipResponseWriter struct {
	http.ResponseWriter
	gzip *gzip.Writer
}

func (w *GzipResponseWriter) Write(p []byte) (int, error) {
	return w.gzip.Write(p)
}

func (w *GzipResponseWriter) Close() (error) {
	return w.gzip.Close()
}

func CanAcceptsGzip(r *http.Request) (bool) {
	s := strings.ToLower(r.Header.Get("Accept-Encoding"))
	for _, ss := range strings.Split(s, ",") {
		if strings.HasPrefix(ss, "gzip") {
			return true
		}
	}
	return false
}

func TryGzipResponse(w http.ResponseWriter, r *http.Request) (*GzipResponseWriter, bool) {
	if !CanAcceptsGzip(r) || GZIP_LV == 0 {
		return nil, false
	}

	gw, err := gzip.NewWriterLevel(w, GZIP_LV)
	if err != nil {
		gw = gzip.NewWriter(w)
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")

	return &GzipResponseWriter{w, gw}, true
}

