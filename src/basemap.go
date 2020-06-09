package webmap

import (
	"encoding/json"
	"sync"
	"sync/atomic"
)

type MapID = uint64
type BaseMap struct {
	ID MapID `json:"mid"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	Hide bool `json:"hide,omitempty"`

	Attribution string `json:"attr,omitempty"`
	Url string `json:"url,omitempty"`
	SubDomain string `json:"subdomains,omitempty"`
	ErrTile string `json:"errorTileUrl,omitempty"`
	MaxZoom int `json:"maxZoom"`
}

func (s *BaseMap) Clone() *BaseMap {
	s2 := *s
	return &s2
}


// assume BaseMap less than 100k, put in RAM
type MapStore struct {
	mx sync.RWMutex
	list map[MapID]*BaseMap
	olist []*BaseMap
	nextID uint64

	slist atomic.Value //[]*BaseMap // cache for api output
}

func NewMapStore() *MapStore {
	s := &MapStore {
		list: make(map[MapID]*BaseMap),
		olist: make([]*BaseMap, 0, 8),
		nextID: 1,
	}
	return s
}

func (s *MapStore) MarshalJSON() ([]byte, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	data := struct{
		Data  []*BaseMap  `json:"data"`
		Next  uint64         `json:"next"`
	}{
		Data: s.olist,
		Next: s.nextID,
	}

	return json.Marshal(data)
}
func (s *MapStore) UnmarshalJSON(in []byte) error {
	data := struct{
		Data  []*BaseMap  `json:"data"`
		Next  uint64         `json:"next"`
	}{}
	err := json.Unmarshal(in, &data)
	if err != nil {
		return err
	}
	if data.Next == 0 { // should not be 0
		data.Next = 1
	}

	s.mx.Lock()
	defer s.mx.Unlock()
	s.nextID = data.Next
	s.olist = data.Data
	s.list = make(map[MapID]*BaseMap, len(data.Data))
	for _, obj := range data.Data {
		id := obj.ID
		if s.list[id] != nil {
			panic("duplicate ID in MapStore!!  broken data?")
			return nil
		}
		s.list[id] = obj
	}

	s.updateCachedList()

	return nil
}

func (s *MapStore) Add(obj *BaseMap) (MapID, error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	next := s.nextID
	id := MapID(next)
	if s.list[id] != nil {
		panic("duplicate ID in MapStore!!  overflow?")
		return 0, ErrItemExist
	}

	obj.ID = id
	s.list[id] = obj
	s.olist = append(s.olist, obj)
	s.nextID += 1

	s.updateCachedList()

	return id, nil
}

func (s *MapStore) Order(ids []MapID) {
	s.mx.Lock()
	defer s.mx.Unlock()

	sz := len(s.list)
	out := make([]*BaseMap, 0, sz)
	for _, id := range ids {
		obj, ok := s.list[id]
		if !ok { // id not exist
			continue
		}
		out = append(out, obj)
	}
	if len(out) != sz { // missing some obj
		m0 := make(map[*BaseMap]*BaseMap, len(out))
		for _, obj := range out {
			m0[obj] = obj
		}
		for _, obj := range s.list {
			if obj != m0[obj] { // not exist
				out = append(out, obj)
			}
		}
	}
	s.olist = out

	s.updateCachedList()

	if len(s.olist) != len(s.list) {
		panic("olist and list should content same object!!")
	}
}

func (s *MapStore) Set(obj *BaseMap) error { // replace by ID
	s.mx.Lock()
	defer s.mx.Unlock()

	id := obj.ID
	if s.list[id] == nil {
		return ErrNotExist
	}
	s.list[id] = obj

	olist := make([]*BaseMap, 0, len(s.olist))
	for _, o := range s.olist {
		if o.ID == id {
			o = obj
		}
		olist = append(olist, o)
	}
	s.olist = olist

	s.updateCachedList()

	return nil
}

func (s *MapStore) Del(id MapID) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.list[id] == nil {
		return nil
	}
	delete(s.list, id)

	olist := make([]*BaseMap, 0, len(s.olist) - 1)
	for _, o := range s.olist {
		if o.ID == id {
			continue
		}
		olist = append(olist, o)
	}
	s.olist = olist

	s.updateCachedList()

	return nil
}

func (s *MapStore) GetByID(id MapID) *BaseMap {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.list[id]
}

func (s *MapStore) GetAll() []*BaseMap {
	s.mx.RLock()
	defer s.mx.RUnlock()

	out := make([]*BaseMap, 0, len(s.olist))
	for _, obj := range s.olist {
		out = append(out, obj.Clone())
	}

	return out
}

func (s *MapStore) GetPub() []*BaseMap { // load cache
	out, ok := s.slist.Load().([]*BaseMap)
	if !ok {
		return nil
	}
	return out
}

func (s *MapStore) updateCachedList() {
	out := make([]*BaseMap, 0, len(s.olist))
	for _, obj := range s.olist {
		if obj.Hide {
			continue
		}
		obj2 := obj.Clone()
		obj2.Note = ""
		out = append(out, obj2)
	}

	s.slist.Store(out)
}

