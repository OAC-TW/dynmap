package webmap

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	//"io"
	"net/http"
	//"sync/atomic"
	"time"
)


type WebCacheResp struct {
	t time.Time
	etag string

	buf []byte
	//gzBuf []byte // TODO
}

func NewCache(data []byte) *WebCacheResp {
	t := now()
	hash := md5.Sum(data)
	etag := hex.EncodeToString(hash[:])
	r := &WebCacheResp{
		t: t,
		buf: data,
		etag: etag,
	}
	return r
}

func (c *WebCacheResp) WriteCache(w http.ResponseWriter, r *http.Request, fileName string) bool {
	if r.Method != "GET" {
		return false
	}
	w.Header().Set("Etag", `"` + c.etag + `"`)
	http.ServeContent(w, r, fileName, c.t, bytes.NewReader(c.buf))
	return true
}

