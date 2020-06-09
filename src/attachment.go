package webmap

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"sort"
	"time"
)

type AttachID = uint64

// put in db
type Attachment struct {
	ID AttachID `json:"aid,omitempty"` // unique
	Token string `json:"token"` // unique

	UploadUID UserID `json:"uid,omitempty"`

	OriginalName string `json:"on"`
	Size int64 `json:"sz"`
	UploadTime time.Time `json:"time"` // also for Last-Modified
	Checksum string `json:"hash"` // also for ETag

	SaveName string `json:"sn,omitempty"` // time + random + hash

	Hide bool `json:"hide,omitempty"` // mark as delete

//	Gzipped bool `json:"gzip,omitempty"` // gzipped on disk
//	GzSize int64 `json:"gzsz,omitempty"`
//	GzChecksum string `json:"gzhash,omitempty"`
}

func (s *Attachment) Clone() *Attachment {
	s2 := *s
	return &s2
}

func (a *Attachment) DelFromFS(baseDir string) error {
	saveFp := filepath.Join(baseDir, a.SaveName)
	return os.Remove(saveFp)
}

func (a *Attachment) ServeContent(w http.ResponseWriter, r *http.Request, baseDir string) {
	saveName := filepath.Clean("/" + a.SaveName)[1:] // clean again for SaveName in db tamper by other program
	saveFp := filepath.Join(baseDir, saveName)

	fi, err := os.Stat(saveFp)
	if err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}
	szCk := a.Size
//	if a.Gzipped {
//		szCk = a.GzSize
//	}
	if fi.Size() != szCk {
		http.Error(w, "419 Checksum failed", 419)
		return
	}

	fd, err := os.OpenFile(saveFp, os.O_RDONLY, 0400)
	if err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}
	defer fd.Close()

	// checksum check
	sha256, ok := sha256fd(fd)
	if !ok || sha256 != a.Checksum {
		http.Error(w, "419 Checksum failed", 419)
		return
	}
	w.Header().Set("Etag", `"` + a.Checksum + `"`)
	http.ServeContent(w, r, a.OriginalName, a.UploadTime, fd)
}

// Token & ID fill by ADD() api
func NewAttachment(name string, size int64) *Attachment {
	now := time.Now()
//	sname := truncate2Sec(now).Format(time.RFC3339) + "-" + genSaveName()
	sname := formatTimestamp(now) + "-" + genSaveName()
	a := &Attachment{
		OriginalName: filepath.Clean("/" + name)[1:], // remove '../'
		Size: size,
		UploadTime: now,
		SaveName: sname,
	}
	return a
}

func genSaveName() string {
	buf, err := genRandomBytes(12)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// assume Attachment less than 5MB, put in RAM
type AttachStore struct {
	mx sync.RWMutex
	list map[AttachID]*Attachment
	lut  map[string]*Attachment // for download
	nextID uint64

	slist atomic.Value //[]*Attachment // cache for api output
}

func NewAttachStore() *AttachStore {
	s := &AttachStore {
		list: make(map[AttachID]*Attachment),
		lut: make(map[string]*Attachment),
		nextID: 1,
	}
	return s
}

func (ls *AttachStore) MarshalJSON() ([]byte, error) {
	ls.mx.RLock()
	defer ls.mx.RUnlock()

	data := struct{
		Data  []*Attachment  `json:"data"`
		Next  uint64         `json:"next"`
	}{
		Data: make([]*Attachment, 0, len(ls.list)),
		Next: ls.nextID,
	}

	for _, obj := range ls.list {
		data.Data = append(data.Data, obj)
	}

	return json.Marshal(data)
}
func (ls *AttachStore) UnmarshalJSON(in []byte) error {
	data := struct{
		Data  []*Attachment  `json:"data"`
		Next  uint64         `json:"next"`
	}{}
	err := json.Unmarshal(in, &data)
	if err != nil {
		return err
	}
	if data.Next == 0 { // should not be 0
		data.Next = 1
	}

	ls.mx.Lock()
	defer ls.mx.Unlock()
	ls.nextID = data.Next
	ls.list = make(map[AttachID]*Attachment, len(data.Data))
	ls.lut = make(map[string]*Attachment, len(data.Data))
	for _, obj := range data.Data {
		id := obj.ID
		if ls.list[id] != nil {
			panic("duplicate ID in AttachStore!!  broken data?")
		}
		token := obj.Token
		if ls.lut[token] != nil {
			panic("duplicate Token in AttachStore!!  broken data?")
		}
		ls.list[id] = obj
		ls.lut[token] = obj
	}

	ls.updateSortList()

	return nil
}

func (ls *AttachStore) Add(obj *Attachment) (AttachID, error) {
	ls.mx.Lock()
	defer ls.mx.Unlock()

	next := ls.nextID
	id := AttachID(next)
	if ls.list[id] != nil {
		panic("duplicate ID in AttachStore!!  overflow?")
		return 0, ErrItemExist
	}

	obj.Token = ""
	for i:=0; i<10000; i++ {
		token := genToken()
		if token == "" { // Not enough entropy to generate random?
			time.Sleep(20 * time.Millisecond)
			continue
		}
		_, ok := ls.lut[token]
		if !ok {
			obj.Token = token
			break
		}
	}
	if obj.Token == "" {
		return 0, ErrTokenGen
	}

	obj.ID = id
	ls.list[id] = obj
	ls.lut[obj.Token] = obj
	ls.nextID += 1

	ls.updateSortList()

	return id, nil
}

func (s *AttachStore) Set(obj *Attachment) error { // replace by AID & Token, AID & Token should not change
	s.mx.Lock()
	defer s.mx.Unlock()

	id := obj.ID
	if s.list[id] == nil {
		return ErrNotExist
	}

	token := obj.Token
	if s.lut[token] == nil {
		return ErrNotExist
	}

	s.list[id] = obj
	s.lut[token] = obj

	s.updateSortList()

	return nil
}


func (s *AttachStore) Del(id AttachID) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	obj, ok := s.list[id]
	if !ok {
		return nil
	}
	delete(s.list, id)
	delete(s.lut, obj.Token)

	s.updateSortList()

	return nil
}

func (s *AttachStore) GetByID(id AttachID) *Attachment {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.list[id]
}

func (s *AttachStore) GetByToken(token string) *Attachment {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.lut[token]
}

func (s *AttachStore) GetAll() []*Attachment {
	s.mx.RLock()
	defer s.mx.RUnlock()

	out := make([]*Attachment, 0, len(s.list))
	for _, obj := range s.list {
		out = append(out, obj.Clone())
	}

	return out
}

func (s *AttachStore) GetWeb() []*Attachment { // load cache
	out, ok := s.slist.Load().([]*Attachment)
	if !ok {
		return nil
	}
	return out
}

type attachByID []*Attachment
func (s attachByID) Len() int      { return len(s) }
func (s attachByID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s attachByID) Less(i, j int) bool { return s[i].ID < s[j].ID }

func (s *AttachStore) updateSortList() { // sort by AID
	out := make([]*Attachment, 0, len(s.list))
	for _, obj := range s.list {
		if obj.Hide {
			continue
		}
		obj2 := obj.Clone()
		//obj2.ID = 0
		obj2.SaveName = ""
		out = append(out, obj2)
	}

	sort.Sort(sort.Reverse(attachByID(out)))
	s.slist.Store(out)
}

