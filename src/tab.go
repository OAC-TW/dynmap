package webmap

import (
	"encoding/json"
	"sync"
	"sync/atomic"
)

type TabID = uint64
type TabData struct {
	ID TabID `json:"tbid"`

	Title string `json:"title,omitempty"`

	Show bool `json:"show,omitempty"`
	Note string `json:"note,omitempty"`

	Icon string `json:"icon,omitempty"`
	CloseIcon string `json:"cicon,omitempty"`

	Data string `json:"data,omitempty"` // Delta
}

func (s *TabData) Clone() *TabData {
	s2 := *s
	return &s2
}


// assume TabData less than 100k, put in RAM
type TabStore struct {
	mx sync.RWMutex
	list map[TabID]*TabData
	olist []*TabData
	nextID uint64

	slist atomic.Value //[]*TabData // cache for api output
}

func NewTabStore() *TabStore {
	s := &TabStore {
		list: make(map[TabID]*TabData),
		olist: make([]*TabData, 0, 8),
		nextID: 1,
	}
	return s
}

func (s *TabStore) MarshalJSON() ([]byte, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	data := struct{
		Data  []*TabData  `json:"data"`
		Next  uint64         `json:"next"`
	}{
		Data: s.olist,
		Next: s.nextID,
	}

	return json.Marshal(data)
}
func (s *TabStore) UnmarshalJSON(in []byte) error {
	data := struct{
		Data  []*TabData  `json:"data"`
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
	s.list = make(map[TabID]*TabData, len(data.Data))
	for _, obj := range data.Data {
		id := obj.ID
		if s.list[id] != nil {
			panic("duplicate ID in TabStore!!  broken data?")
			return nil
		}
		s.list[id] = obj
	}

	s.updateCachedList()

	return nil
}

func (s *TabStore) Add(obj *TabData) (TabID, error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	next := s.nextID
	id := TabID(next)
	if s.list[id] != nil {
		panic("duplicate ID in TabStore!!  overflow?")
		return 0, ErrItemExist
	}

	obj.ID = id
	s.list[id] = obj
	s.olist = append(s.olist, obj)
	s.nextID += 1

	s.updateCachedList()

	return id, nil
}

func (s *TabStore) Order(ids []TabID) {
	s.mx.Lock()
	defer s.mx.Unlock()

	sz := len(s.list)
	out := make([]*TabData, 0, sz)
	for _, id := range ids {
		obj, ok := s.list[id]
		if !ok { // id not exist
			continue
		}
		out = append(out, obj)
	}
	if len(out) != sz { // missing some obj
		m0 := make(map[*TabData]*TabData, len(out))
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

func (s *TabStore) Set(obj *TabData) error { // replace by ID
	s.mx.Lock()
	defer s.mx.Unlock()

	id := obj.ID
	if s.list[id] == nil {
		return ErrNotExist
	}
	s.list[id] = obj

	olist := make([]*TabData, 0, len(s.olist))
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

func (s *TabStore) Del(id TabID) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.list[id] == nil {
		return nil
	}
	delete(s.list, id)

	olist := make([]*TabData, 0, len(s.olist) - 1)
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

func (s *TabStore) GetByID(id TabID) *TabData {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.list[id]
}

func (s *TabStore) GetAll() []*TabData {
	s.mx.RLock()
	defer s.mx.RUnlock()

	out := make([]*TabData, 0, len(s.olist))
	for _, obj := range s.olist {
		out = append(out, obj.Clone())
	}

	return out
}

func (s *TabStore) GetPub() []*TabData { // load cache
	out, ok := s.slist.Load().([]*TabData)
	if !ok {
		return nil
	}
	return out
}

func (s *TabStore) updateCachedList() {
	out := make([]*TabData, 0, len(s.olist))
	for _, obj := range s.olist {
		if !obj.Show {
			continue
		}
		obj2 := obj.Clone()
		obj2.Note = ""
		out = append(out, obj2)
	}

	s.slist.Store(out)
}

