package webmap

import (
	"encoding/json"
	"sync"
	"sync/atomic"
)

type LayerID = uint64
type LayerGroup struct {
	ID LayerID `json:"lyid"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	Hide bool `json:"hide,omitempty"`
	Show bool `json:"show,omitempty"` // show when open

	Attribution string `json:"attr,omitempty"`
	Token string `json:"token"` // for serve path, static '/dl/', dynamic '/hook/'
	Color string `json:"color,omitempty"` // color '#3388FF'
	FillColor string `json:"fillcolor,omitempty"` // color '#3388FF'
	Opacity float32 `json:"opacity,omitempty"` // opacity

	UV bool `json:"uv,omitempty"` // show as UV layer (wind map ...etc)
	VelScale float32 `json:"velocityScale,omitempty"` // velocityScale

	Dynamic bool `json:"dyn,omitempty"` // for dynamic data

	//Objs map[ObjID]*LayerObj // for objs
}

func (s *LayerGroup) Clone() *LayerGroup {
	s2 := *s
	return &s2
}


// assume LayerGroup less than 100k, put in RAM
type LayerStore struct {
	mx sync.RWMutex
	list map[LayerID]*LayerGroup
	olist []*LayerGroup
	nextID uint64

	slist atomic.Value //[]*LayerGroup // cache for api output
}

func NewLayerStore() *LayerStore {
	s := &LayerStore {
		list: make(map[LayerID]*LayerGroup),
		olist: make([]*LayerGroup, 0, 8),
		nextID: 1,
	}
	return s
}

func (s *LayerStore) MarshalJSON() ([]byte, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	data := struct{
		Data  []*LayerGroup  `json:"data"`
		Next  uint64         `json:"next"`
	}{
		Data: s.olist,
		Next: s.nextID,
	}

	return json.Marshal(data)
}
func (s *LayerStore) UnmarshalJSON(in []byte) error {
	data := struct{
		Data  []*LayerGroup  `json:"data"`
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
	s.list = make(map[LayerID]*LayerGroup, len(data.Data))
	for _, obj := range data.Data {
		id := obj.ID
		if s.list[id] != nil {
			panic("duplicate ID in LayerStore!!  broken data?")
			return nil
		}
		s.list[id] = obj
	}

	s.updateCachedList()

	return nil
}

func (s *LayerStore) Add(obj *LayerGroup) (LayerID, error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	next := s.nextID
	id := LayerID(next)
	if s.list[id] != nil {
		panic("duplicate ID in LayerStore!!  overflow?")
		return 0, ErrItemExist
	}

	obj.ID = id
	s.list[id] = obj
	s.olist = append(s.olist, obj)
	s.nextID += 1

	s.updateCachedList()

	return id, nil
}

func (s *LayerStore) Order(ids []LayerID) {
	s.mx.Lock()
	defer s.mx.Unlock()

	sz := len(s.list)
	out := make([]*LayerGroup, 0, sz)
	for _, id := range ids {
		obj, ok := s.list[id]
		if !ok { // id not exist
			continue
		}
		out = append(out, obj)
	}
	if len(out) != sz { // missing some obj
		m0 := make(map[*LayerGroup]*LayerGroup, len(out))
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

func (s *LayerStore) Set(obj *LayerGroup) error { // replace by ID
	s.mx.Lock()
	defer s.mx.Unlock()

	id := obj.ID
	if s.list[id] == nil {
		return ErrNotExist
	}
	s.list[id] = obj

	olist := make([]*LayerGroup, 0, len(s.olist))
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

func (s *LayerStore) Del(id LayerID) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.list[id] == nil {
		return nil
	}
	delete(s.list, id)

	olist := make([]*LayerGroup, 0, len(s.olist) - 1)
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

func (s *LayerStore) GetByID(id LayerID) *LayerGroup {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.list[id]
}

func (s *LayerStore) GetAll() []*LayerGroup {
	s.mx.RLock()
	defer s.mx.RUnlock()

	out := make([]*LayerGroup, 0, len(s.olist))
	for _, obj := range s.olist {
		out = append(out, obj.Clone())
	}

	return out
}

func (s *LayerStore) GetPub() []*LayerGroup { // load cache
	out, ok := s.slist.Load().([]*LayerGroup)
	if !ok {
		return nil
	}
	return out
}

func (s *LayerStore) updateCachedList() {
	out := make([]*LayerGroup, 0, len(s.olist))
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

