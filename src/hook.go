package webmap

/*
* simple cache system for external process input
* eg: data pipeline for wind map, ocean current
* set cache by web POST (no resume)
* get cache by web GET (can resume)
*/

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"sort"
	"time"
)

var (
	CacheInMemorySizeLimit = int64(16 * 1024 * 1024) // Bytes (16 MB)
	CacheFileSizeLimit = int64(128 * 1024 * 1024) // Bytes (128 MB)
	CacheFileDir = "./cache/"
)

type HookID = uint64

// put in db
type HookConfig struct {
	// set once
	ID HookID `json:"hid,omitempty"` // unique
	Token string `json:"token"` // unique, for guest download
	AuthToken string `json:"auth"` // unique, for data pipeline input

	// set when data input
	Size int64 `json:"sz"`
	UpdateTime time.Time `json:"time"` // also for Last-Modified
	Checksum string `json:"hash"` // also for ETag
	SaveName string `json:"sn,omitempty"` // time + random + hash
	ExtName string `json:"ext,omitempty"`
	cache atomic.Value //[]byte // in-memory cache for small file

	// set by config
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	Disable bool `json:"disable,omitempty"`
	RenderType string `json:"type,omitempty"` // geojson, UV json, UV png, UV bin
	// TODO: limit source IP? only set by another https server bind on different IP/port?
}

func (s *HookConfig) Clone() *HookConfig {
	s2 := *s
	return &s2
}

func (a *HookConfig) DelFromFS(baseDir string) error {
	saveFp := filepath.Join(baseDir, a.SaveName)
	return os.Remove(saveFp)
}

func (a *HookConfig) ServeContent(w http.ResponseWriter, r *http.Request, baseDir string) {
	szCk := a.Size
	if szCk < CacheInMemorySizeLimit { // check in-memory cache first
		buf, ok := a.cache.Load().([]byte)
		if ok && len(buf) == int(szCk) { // cache valid
			w.Header().Set("Etag", `"` + a.Checksum + `"`)
			http.ServeContent(w, r, a.ExtName, a.UpdateTime, bytes.NewReader(buf))
			return
		}
	}

	saveName := filepath.Clean("/" + a.SaveName)[1:] // clean again for SaveName in db tamper by other program
	saveFp := filepath.Join(baseDir, saveName)

	fi, err := os.Stat(saveFp)
	if err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}
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

	// TODO: if size < in-memory size limit, cache it

	// checksum check
	sha256, ok := sha256fd(fd)
	if !ok || sha256 != a.Checksum {
		http.Error(w, "419 Checksum failed", 419)
		return
	}
	w.Header().Set("Etag", `"` + a.Checksum + `"`)
	http.ServeContent(w, r, a.ExtName, a.UpdateTime, fd)
}

// only update info about data
func (a *HookConfig) SetData(name string, size int64) *HookConfig {
	now := time.Now()
//	sname := truncate2Sec(now).Format(time.RFC3339) + "-" + genSaveName()
	sname := formatTimestamp(now) + "-" + genSaveName()

	a.ExtName = filepath.Clean("/" + name)[1:] // remove '../'
	a.Size = size
	a.UpdateTime = now
	a.SaveName = sname
	return a
}

// assume HookConfig less than 10MB, put in RAM
type HookStore struct {
	mx sync.RWMutex
	list map[HookID]*HookConfig
	lut  map[string]*HookConfig // for download
	lutA map[string]*HookConfig // for data pipeline input
	nextID uint64

	slist atomic.Value //[]*HookConfig // cache for api output
}

func NewHookStore() *HookStore {
	s := &HookStore {
		list: make(map[HookID]*HookConfig),
		lut: make(map[string]*HookConfig),
		lutA: make(map[string]*HookConfig),
		nextID: 1,
	}
	return s
}

func (ls *HookStore) MarshalJSON() ([]byte, error) {
	ls.mx.RLock()
	defer ls.mx.RUnlock()

	data := struct{
		Data  []*HookConfig  `json:"data"`
		Next  uint64         `json:"next"`
	}{
		Data: make([]*HookConfig, 0, len(ls.list)),
		Next: ls.nextID,
	}

	for _, obj := range ls.list {
		data.Data = append(data.Data, obj)
	}

	return json.Marshal(data)
}
func (ls *HookStore) UnmarshalJSON(in []byte) error {
	data := struct{
		Data  []*HookConfig  `json:"data"`
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
	ls.list = make(map[HookID]*HookConfig, len(data.Data))
	ls.lut = make(map[string]*HookConfig, len(data.Data))
	for _, obj := range data.Data {
		id := obj.ID
		if ls.list[id] != nil {
			panic("duplicate ID in HookStore!!  broken data?")
		}
		token := obj.Token
		if ls.lut[token] != nil {
			panic("duplicate Token in HookStore!!  broken data?")
		}
		tokenAuth := obj.AuthToken
		if ls.lutA[tokenAuth] != nil {
			panic("duplicate AuthToken in HookStore!!  broken data?")
		}
		ls.list[id] = obj
		ls.lut[token] = obj
		ls.lutA[tokenAuth] = obj
	}

	ls.updateSortList()

	return nil
}

func (ls *HookStore) Add(obj *HookConfig) (HookID, error) {
	ls.mx.Lock()
	defer ls.mx.Unlock()

	next := ls.nextID
	id := HookID(next)
	if ls.list[id] != nil {
		panic("duplicate ID in HookStore!!  overflow?")
		return 0, ErrItemExist
	}

	token := mkHookToken(ls.lut)
	if token == "" {
		return 0, ErrTokenGen
	}
	obj.Token = token

	tokenAuth := mkHookToken(ls.lutA)
	if tokenAuth == "" {
		return 0, ErrTokenGen
	}
	obj.AuthToken = tokenAuth

	obj.ID = id
	ls.list[id] = obj
	ls.lut[obj.Token] = obj
	ls.lutA[obj.AuthToken] = obj
	ls.nextID += 1

	ls.updateSortList()

	return id, nil
}

func mkHookToken(lut map[string]*HookConfig) string {
	for i:=0; i<10000; i++ {
		token := genToken()
		if token == "" { // Not enough entropy to generate random?
			time.Sleep(20 * time.Millisecond)
			continue
		}
		_, ok := lut[token]
		if !ok {
			return token
		}
	}
	return ""
}

func (s *HookStore) SetConfig(obj *HookConfig) error { // replace by HID, should only change config value
	s.mx.Lock()
	defer s.mx.Unlock()

	id := obj.ID
	obj0, ok := s.list[id]
	if !ok {
		return ErrNotExist
	}

	// copy to original obj
	obj0.Name = obj.Name
	obj0.Note = obj.Note
	obj0.Disable = obj.Disable
	obj0.RenderType = obj.RenderType

	s.updateSortList()

	return nil
}

func (s *HookStore) Set(obj *HookConfig) error { // replace by HID, should only change config value
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

	tokenAuth := obj.AuthToken
	if s.lutA[tokenAuth] == nil {
		return ErrNotExist
	}

	s.list[id] = obj
	s.lut[token] = obj
	s.lutA[tokenAuth] = obj

	s.updateSortList()

	return nil
}


func (s *HookStore) Del(id HookID) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	obj, ok := s.list[id]
	if !ok {
		return nil
	}
	delete(s.list, id)
	delete(s.lut, obj.Token)
	delete(s.lutA, obj.AuthToken)

	s.updateSortList()

	return nil
}

func (s *HookStore) GetByID(id HookID) *HookConfig {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.list[id]
}

func (s *HookStore) GetByToken(token string) *HookConfig {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.lut[token]
}

func (s *HookStore) GetByAuthToken(token string) *HookConfig {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.lutA[token]
}

func (s *HookStore) GetAll() []*HookConfig {
	s.mx.RLock()
	defer s.mx.RUnlock()

	out := make([]*HookConfig, 0, len(s.list))
	for _, obj := range s.list {
		out = append(out, obj.Clone())
	}

	return out
}

func (s *HookStore) GetWeb() []*HookConfig { // load cache
	out, ok := s.slist.Load().([]*HookConfig)
	if !ok {
		return nil
	}
	return out
}

type hookByID []*HookConfig
func (s hookByID) Len() int      { return len(s) }
func (s hookByID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s hookByID) Less(i, j int) bool { return s[i].ID < s[j].ID }

func (s *HookStore) updateSortList() { // sort by HID
	out := make([]*HookConfig, 0, len(s.list))
	for _, obj := range s.list {
		obj2 := obj.Clone()
		obj2.SaveName = ""
		out = append(out, obj2)
	}

	sort.Sort(sort.Reverse(hookByID(out)))
	s.slist.Store(out)
}

