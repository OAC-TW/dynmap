package webmap

import (
	"crypto/sha256"
 	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	mrand "math/rand"
	//"fmt"
	//"log"
	"io"
	"sync"
	"time"
)

/*var Verbosity = 3

func Vf(level int, format string, v ...interface{}) {
	if level <= Verbosity {
		log.Printf(format, v...)
	}
}
func Vln(level int, v ...interface{}) {
	if level <= Verbosity {
		log.Println(v...)
	}
}*/

type RateLimit struct {
	mx   sync.Mutex
	t    time.Time
	dt   time.Duration
	dirty bool
}

func (r *RateLimit) IsDirty() bool {
	r.mx.Lock()
	defer r.mx.Unlock()
	return r.dirty
}

func (r *RateLimit) SetDirty() {
	r.mx.Lock()
	r.dirty = true
	r.mx.Unlock()
}

func (r *RateLimit) CanFlush() bool {
	r.mx.Lock()
	defer r.mx.Unlock()

	now := time.Now()
	if now.After(r.t) {
		r.t = now.Add(r.dt)
		r.dirty = false
		return true
	}
	r.dirty = true
	return false
}

func NewRateLimit(dt time.Duration) *RateLimit {
	return &RateLimit{
		t: time.Now(),
		dt: dt,
	}
}

func getNowSec() time.Time {
	return truncate2Sec(time.Now())
}

func truncate2Sec(t time.Time) time.Time {
	return t.UTC().Truncate(time.Second)
}

func formatTimestamp(t time.Time) string {
	return t.Format("20060102T150405") // "2006-01-02T15:04:05Z07:00"
}

func sha256fd(f io.Reader) (string, bool) {
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err.Error(), false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

func cpAndHashFd(fo io.Writer, fi io.Reader) (string, error) {
	sha1h := sha256.New()
	w := io.MultiWriter(fo, sha1h)
	if _, err := io.Copy(w, fi); err != nil {
		return "", err
	}
	return hex.EncodeToString(sha1h.Sum(nil)), nil
}

func isExeFile(fd io.ReadSeeker) (ok bool) {
	defer fd.Seek(0, 0)
	
	buf := make([]byte, 64)

	_, err := io.ReadFull(fd, buf[:10])
	if err != nil {
		return
	}
	if binary.LittleEndian.Uint32(buf[0:4]) == 0x464C457F && binary.LittleEndian.Uint32(buf[6:]) == 0x01 { // ELF 0x7f, 'ELF'
		return true
	}


	fd.Seek(0, 0)
	_, err = io.ReadFull(fd, buf[:64])
	if err != nil {
		return
	}

	if !(buf[0] == 0x4D && buf[1] == 0x5A) { // DOS 'MZ'
		return
	}

	peOff := binary.LittleEndian.Uint32(buf[60:])
	if peOff == 0 {
		return true // DOS?
	}
	_, err = fd.Seek(int64(peOff), 0)
	if err != nil {
		return true // DOS?
	}
	_, err = io.ReadFull(fd, buf[:4])
	if err != nil {
		return true // DOS?
	}
	if binary.LittleEndian.Uint32(buf[:4]) == 0x4550 { // 'PE', 0x00, 0x00
		return true // PE
	}

	return true // broken PE?
}

func genRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func genToken() string {
	buf, err := genRandomBytes(15)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func genRng8() string {
	buf, err := genRandomBytes(6)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func init() {
	mrand.Seed(time.Now().UnixNano())
}

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ~!@#$%^&*_-+="
func GenPWD(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[mrand.Intn(len(letterBytes))]
	}
	return string(b)
}

