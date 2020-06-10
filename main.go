package main

import (
	"crypto/tls"
	"net/http"
	"flag"
	"log"
	"time"

	"context"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"errors"

	webmap "local/webmap"
)

var (
	gzipLv = flag.Int("gz", 5, "gzip disable = 0, DefaultCompression = -1, BestSpeed = 1, BestCompression = 9")

	upto = flag.Int("upto", 10*60, "upload timeout in Seconds")
	dwto = flag.Int("dwto", 10*60, "download timeout in Seconds")

	addr = flag.String("l", ":4040", "bind addr & port")
	crtFile = flag.String("crt", "", "https certificate file")
	keyFile = flag.String("key", "", "https private key file")

	verbosity = flag.Int("v", 3, "verbosity for app")
	logDir = flag.String("logdir", "log/", "path to save log file")
	logFile = flag.String("log", "access-%v.log", "access log file for all http request, '%v' for date")
	syslogFile = flag.String("syslog", "system-%v.log", "system log file, '%v' for date")

	dbFile = flag.String("db", "webmap.db", "json database file")

	ssusr = flag.String("ssusr", "", "temporary super user login acc")
)

func main() {
	flag.Parse()

	if err := createDirIfNotExist(*logDir); err != nil {
		log.Println("[dir]create", *logDir, err)
		return
	}

	if err := createDirIfNotExist("./upload"); err != nil { // TODO: not hardcode
		log.Println("[dir]create", "upload", err)
		return
	}
	if err := createDirIfNotExist("./cache"); err != nil { // TODO: not hardcode
		log.Println("[dir]create", "cache", err)
		return
	}

	db := webmap.NewDataStore()
	err := db.Open(*dbFile)
	if err != nil {
		log.Println("[db]error", err)
		return
	}

	if *ssusr != "" {
		pwd := webmap.GenPWD(10)
		db.AddShadowUser(*ssusr, pwd)
		log.Println("[warn]enable temporary super user:", *ssusr, pwd)
	}

	webmap.GZIP_LV = *gzipLv
	webmap.Verbosity = *verbosity
	webmap.SetFileOutput(filepath.Join(*logDir, *syslogFile))
	webmap.SetWebOutput(filepath.Join(*logDir, *logFile))


	web := webmap.NewWebAPI(db)
	web.Handle("/admin/", http.FileServer(NewSPADir("./www", "./www/admin/index.html", "admin")))
	web.Handle("/res/", webmap.ReqGz(webmap.ReqCache(http.StripPrefix("/res/", http.FileServer(http.Dir("./www/res"))), "public, no-cache, max-age=0, must-revalidate")))
//	web.Handle("/res/", http.StripPrefix("/res/", http.FileServer(http.Dir("./www/res"))))
	srv := &http.Server{
		ReadTimeout: time.Duration(*upto) * time.Second,
		WriteTimeout: time.Duration(*dwto) * time.Second,
		Addr: *addr,
		Handler: webmap.ReqLog(web),
		ReadHeaderTimeout: 20 * time.Second,
		IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1024*1024, // 1MB
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	startServer(srv, *crtFile, *keyFile)

	<-idleConnsClosed

	// flush & exit
	db.Close()
}

func startServer(srv *http.Server, crt string, key string) {
	var err error

	// check tls
	if crt != "" && key != "" {
		cfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{

				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,

				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,

				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, // http/2 must
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, // http/2 must

				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,

				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

				tls.TLS_RSA_WITH_AES_256_GCM_SHA384, // weak
				tls.TLS_RSA_WITH_AES_256_CBC_SHA, // waek
			},
		}
		srv.TLSConfig = cfg
		//srv.TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0) // disable http/2

		log.Printf("[server] HTTPS server Listen on: %v", srv.Addr)
		err = srv.ListenAndServeTLS(crt, key)
	} else {
		log.Printf("[server] HTTP server Listen on: %v", srv.Addr)
		err = srv.ListenAndServe()
	}

	if err != http.ErrServerClosed {
		log.Printf("[server] ListenAndServe error: %v", err)
	}
}

// An empty Dir is treated as ".".
type SPADir struct {
	dir string
	base string
	//tag string
}

func NewSPADir(dir string, base string, tag string) SPADir {
	return SPADir{
		dir: dir,
		base: base,
		//tag: tag,
	}
}

func mapDirOpenError(originalErr error, name string) error {
	if os.IsNotExist(originalErr) || os.IsPermission(originalErr) {
		return originalErr
	}

	parts := strings.Split(name, string(filepath.Separator))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		fi, err := os.Stat(strings.Join(parts[:i+1], string(filepath.Separator)))
		if err != nil {
			return originalErr
		}
		if !fi.IsDir() {
			return os.ErrNotExist
		}
	}
	return originalErr
}

func (d SPADir) Open(name string) (http.File, error) {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return nil, errors.New("http: invalid character in file path")
	}
	dir := d.dir
	if dir == "" {
		dir = "."
	}
	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	f, err := os.Open(fullName)
	if err != nil {
		err2 := mapDirOpenError(err, fullName)
		if strings.HasSuffix(name, "/index.html") { // try open index.html or list dir
			return nil, err2
		}
		if err2 != os.ErrNotExist {
			f2, err := os.Open(d.base)
			if err != nil {
				return nil, err2
			}
			f = f2
		}
	}
	return f, nil
}

func createDirIfNotExist(dirName string) error {
	src, err := os.Stat(dirName)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(dirName, 0755)
		if errDir != nil {
			return err
		}
		return nil
	}

	if src.Mode().IsRegular() {
		return errors.New("already exist as a file!")
	}

	return nil
}

