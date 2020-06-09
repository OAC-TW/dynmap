package webmap

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"sort"

	"golang.org/x/crypto/bcrypt"
)

const (
	_HASH_COST_FACTOR = 12 // cost factor need change with server performance & time
)

type UserID = uint64

// put in db
type User struct {
	ID UserID `json:"uid"` // unique

	Acc string `json:"acc"` // unique
	Hash string `json:"hash,omitempty"`

	Name string `json:"name"`
	Note string `json:"note,omitempty"`

	Super bool `json:"su,omitempty"` // can edit other user?

	Freeze bool `json:"fz,omitempty"`
}

func (s *User) Clone() *User {
	s2 := *s
	return &s2
}

func (u *User) SetPasswd(pwd string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pwd), _HASH_COST_FACTOR)
	if err != nil {
		return err
	}
	u.Hash = string(bytes)
	return nil
}

func (u *User) CheckPasswd(pwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(pwd))
	return err == nil
}

// assume all User count less than 1k, put in RAM
type UserStore struct {
	mx sync.RWMutex
	list map[UserID]*User
	acc  map[string]*User // for login
	nextID uint64

	slist atomic.Value //[]*User // cache for api output
}


func NewUserStore() *UserStore {
	s := &UserStore {
		list: make(map[UserID]*User),
		acc: make(map[string]*User),
		nextID: 1,
	}
	return s
}

func (s *UserStore) MarshalJSON() ([]byte, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	data := struct{
		Data  []*User  `json:"data"`
		Next  uint64   `json:"next"`
	}{
		Data: make([]*User, 0, len(s.list)),
		Next: s.nextID,
	}

	for _, obj := range s.list {
		data.Data = append(data.Data, obj)
	}

	return json.Marshal(data)
}
func (s *UserStore) UnmarshalJSON(in []byte) error {
	data := struct{
		Data  []*User  `json:"data"`
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
	s.list = make(map[UserID]*User, len(data.Data))
	for _, obj := range data.Data {
		id := obj.ID
		if s.list[id] != nil {
			panic("duplicate ID in UserStore!!  broken data?")
		}
		s.list[id] = obj

		acc := obj.Acc
		if s.acc[acc] != nil {
			panic("duplicate ACC in UserStore!!  broken data?")
		}
		s.acc[acc] = obj
	}

	s.updateSortList()

	return nil
}

func (s *UserStore) Add(obj *User) (UserID, error) {
	s.mx.Lock()
	defer s.mx.Unlock()

	acc := obj.Acc
	if s.acc[acc] != nil {
		return 0, ErrAccExist
	}

	next := s.nextID
	id := UserID(next)
	if s.list[id] != nil {
		panic("duplicate ID in UserStore!!  overflow?")
		return 0, ErrItemExist
	}

	obj.ID= id
	s.list[id] = obj
	s.acc[acc] = obj
	s.nextID += 1

	s.updateSortList()

	return id, nil
}

func (s *UserStore) Set(obj *User) error { // replace by ID & Acc, ID & Acc should not change
	s.mx.Lock()
	defer s.mx.Unlock()

	id := obj.ID
	if s.list[id] == nil {
		return ErrNotExist
	}

	acc := obj.Acc
	if s.acc[acc] == nil {
		return ErrNotExist
	}

	if obj.Hash == "" {
		u := s.list[id]
		obj.Hash = u.Hash
	}

	s.list[id] = obj
	s.acc[acc] = obj

	s.updateSortList()

	return nil
}

func (s *UserStore) Del(id UserID) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	u, ok := s.list[id]
	if !ok {
		return nil
	}
	delete(s.list, id)
	delete(s.acc, u.Acc)

	s.updateSortList()

	return nil
}

func (s *UserStore) GetByID(id UserID) *User {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.list[id]
}

func (s *UserStore) GetByAcc(acc string) *User {
	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.acc[acc]
}

func (s *UserStore) GetAll() []*User {
	out, ok := s.slist.Load().([]*User)
	if !ok {
		return nil
	}
	return out
}

type userByID []*User
func (s userByID) Len() int      { return len(s) }
func (s userByID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s userByID) Less(i, j int) bool { return s[i].ID < s[j].ID }

func (s *UserStore) updateSortList() { // sort by UID
	out := make([]*User, 0, len(s.list))
	for _, obj := range s.list {
		v := obj.Clone()
		v.Hash = ""
		out = append(out, v)
	}

	sort.Sort(userByID(out))

	s.slist.Store(out)
}

