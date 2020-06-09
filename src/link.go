package webmap

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"fmt"
)

type LinkID = uint64
type Link struct {
	ID LinkID `json:"lkid"`
	Name string `json:"name"` // link name or category name
	Note string `json:"note,omitempty"`
	Hide bool `json:"hide,omitempty"`


	Url string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`


	Indent int `json:"indent,omitempty"`
}

func (s *Link) Clone() *Link {
	s2 := *s
	return &s2
}

type LinkOrder_S struct {
	ID uint64
	LV int
}

func (s *LinkOrder_S) String() string {
	return fmt.Sprintf("<%v/%v>", s.ID, s.LV)
}

func LinkOrder(id LinkID, indent int) *LinkOrder_S {
	return &LinkOrder_S{uint64(id), indent}
}

// assume LinkStore less than 10MB, put in RAM
type LinkStore struct {
	mx sync.RWMutex
	list map[LinkID]*Link // keep all link
	olist []*Link
	nextID uint64

	slist atomic.Value //[]*Link // cache for api output
}

func NewLinkStore() *LinkStore {
	s := &LinkStore {
		list: make(map[LinkID]*Link),
		olist: make([]*Link, 0, 8),
		nextID: 1,
	}
	return s
}

func (s *LinkStore) MarshalJSON() ([]byte, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	data := struct{
		Data  []*Link  `json:"data"`
		Next  uint64   `json:"next"`
	}{
		Data: s.olist,
		Next: s.nextID,
	}

	return json.Marshal(data)
}
func (s *LinkStore) UnmarshalJSON(in []byte) error {
	data := struct{
		Data  []*Link  `json:"data"`
		Next  uint64   `json:"next"`
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
	s.list = make(map[LinkID]*Link, len(data.Data))
	for _, obj := range data.Data {
		id := obj.ID
		if s.list[id] != nil {
			panic("duplicate ID in LinkStore!!  broken data?")
			return nil
		}
		s.list[id] = obj
	}

	s.updateCachedList()

	return nil
}

func (s *LinkStore) Add(obj *Link) (LinkID, error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	next := s.nextID
	id := LinkID(next)
	if s.list[id] != nil {
		panic("duplicate ID in LinkStore!!  overflow?")
		return 0, ErrItemExist
	}

	obj.ID = id
	s.list[id] = obj
	s.olist = append(s.olist, obj)
	s.nextID += 1

	s.updateCachedList()

	return id, nil
}

func (s *LinkStore) Set(obj *Link) error { // replace by ID, only change content
	s.mx.Lock()
	defer s.mx.Unlock()

	id := obj.ID
	oobj, ok := s.list[id]
	if !ok {
		return ErrNotExist
	}

	obj.Indent = oobj.Indent
	s.list[id] = obj

	olist := make([]*Link, 0, len(s.olist))
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

// del all sub? don't change all sub? upper all sub?
func (s *LinkStore) Del(id LinkID) error { // upper all sub
	s.mx.Lock()
	defer s.mx.Unlock()

	obj, ok := s.list[id]
	if !ok {
		return nil
	}
	delete(s.list, id)

	lv := obj.Indent
	upper := false
	olist := make([]*Link, 0, len(s.olist) - 1)
	for _, o := range s.olist {
		if o.ID == id {
			upper = true
			continue
		}
		if upper {
			if o.Indent > lv {
				o.Indent -= 1
			} else {
				upper = false
			}
		}
		olist = append(olist, o)
	}
	s.olist = olist

	s.updateCachedList()

	return nil
}

func (s *LinkStore) Order(ids []*LinkOrder_S) {
	s.mx.Lock()
	defer s.mx.Unlock()

	sz := len(s.list)
	m0 := make(map[*Link]*Link, sz)
	out := make([]*Link, 0, sz)
	for _, o := range ids {
		id := o.ID
		obj, ok := s.list[id]
		if !ok { // id not exist
			continue
		}

		// TODO: verify indent level
		obj.Indent = o.LV
		out = append(out, obj)

		// mark set
		m0[obj] = obj
	}
	if len(m0) != sz { // missing some obj
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

func (s *LinkStore) GetByID(id LinkID) *Link {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.list[id]
}

func (s *LinkStore) GetAll() []*Link {
	s.mx.RLock()
	defer s.mx.RUnlock()

	out := make([]*Link, 0, len(s.olist))
	for _, obj := range s.olist {
		out = append(out, obj.Clone())
	}

	return out
}

func (s *LinkStore) GetPub() []*Link { // load cache
	out, ok := s.slist.Load().([]*Link)
	if !ok {
		return nil
	}
	return out
}

func (s *LinkStore) updateCachedList() {
	out := make([]*Link, 0, len(s.olist))
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

