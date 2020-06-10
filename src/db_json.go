package webmap

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
	"io/ioutil"
	"os"
)

// assume all less than 10MB, put in RAM
type DataStore struct {
	mx sync.RWMutex
	die     chan struct{}

	path string // db file path
	limter *RateLimit
	ssUser map[string]*User

	PageView uint64 `json:"pv,omitempty"` // atomic
	UserVisit uint64 `json:"uv,omitempty"` // atomic

	SiteConfig *TmplIndex `json:"conf,omitempty"`
	config atomic.Value `json:"-"` // *TmplIndex for cached

	User   *UserStore   `json:"user"`
	Attach *AttachStore `json:"attach"`
	Hook   *HookStore `json:"hook"`

	Layer  *LayerStore  `json:"layer"`
	Map    *MapStore    `json:"map"`

	Link   *LinkStore   `json:"link"`
	Tab    *TabStore   `json:"tabs"`
}

func NewDataStore() *DataStore {
	ds := &DataStore {
		die: make(chan struct{}),
		limter: NewRateLimit(30 * time.Second),

		User: NewUserStore(),
		Attach: NewAttachStore(),
		Hook: NewHookStore(),

		Layer: NewLayerStore(),
		Map: NewMapStore(),

		Link: NewLinkStore(),
		Tab: NewTabStore(),

		ssUser: make(map[string]*User, 1),
	}
	return ds
}

func (s *DataStore) Open(dbPath string) error {
	//af, err := os.Open(dbPath)
	af, err := os.OpenFile(dbPath, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		Vln(2, "[db][open]err", err)
		return err
	}
	defer af.Close()

	buf, err := ioutil.ReadAll(af)
	if err != nil {
		Vln(2, "[db][read]err", err)
		return err
	}

	s.mx.Lock()
	defer s.mx.Unlock()

	if len(buf) == 0 { // if file not exist or empty
		goto END
	}

	err = json.Unmarshal(buf, s)
	if err != nil {
		Vln(2, "[db][parse]err", err)
		return err
	}

	// save to atomic value
	s.config.Store(s.SiteConfig)

END:
	s.path = dbPath
	go s.saver()

	return nil
}

func (s *DataStore) Close() error {
	select {
	case <-s.die:
	default:
		close(s.die)
	}
	return s.Flush()
}

func (s *DataStore) Flush() error {
	s.mx.RLock()
	defer s.mx.RUnlock()

	buf, err := json.Marshal(s)
	if err != nil {
		Vln(2, "[db][save]err", err)
		return err
	}

	fp := s.path
	if fp == "" {
		Vln(2, "[db][save]not opened!")
		return nil
	}
	of, err := os.OpenFile(fp, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		Vln(2, "[db][write]err", fp, err)
		return err
	}
	defer of.Close()

	_, err = of.Write(buf)
	return err
}

func (s *DataStore) saver() {
	ticker := time.NewTicker(3 * 60 * time.Second)
	defer ticker.Stop()

	for {
		select{
		case <-s.die:
			return
		case <-ticker.C:
		}
		if s.limter.IsDirty() {
			s.Flush()
		}
	}
}

func (s *DataStore) FlagDirty() {
	s.limter.SetDirty()
}

func (s *DataStore) GetPubLayer() []*LayerGroup {
	return s.Layer.GetPub()
}

func (s *DataStore) GetPubMap() []*BaseMap {
	return s.Map.GetPub()
}

func (s *DataStore) GetPubLink() []*Link {
	return s.Link.GetPub()
}

func (s *DataStore) GetPubTab() []*TabData {
	return s.Tab.GetPub()
}

func (s *DataStore) GetPageView() uint64 {
	return atomic.LoadUint64(&s.PageView)
}
func (s *DataStore) AddAndGetPageView() uint64 {
	return atomic.AddUint64(&s.PageView, uint64(1))
}

func (s *DataStore) GetUserVisit() uint64 {
	return atomic.LoadUint64(&s.UserVisit)
}
func (s *DataStore) AddAndGetUserVisit() uint64 {
	return atomic.AddUint64(&s.UserVisit, uint64(1))
}

func (s *DataStore) GetConfig() *TmplIndex {
	conf, ok := s.config.Load().(*TmplIndex)
	if !ok || conf == nil {
		return &TmplIndex{
			SiteTitle: "map stie",
			Logo: "res/Logo.png",
		}
	}
	return conf
}

func (s *DataStore) SetConfig(conf *TmplIndex) {
	s.mx.Lock()
	s.config.Store(conf)
	s.SiteConfig = conf
	s.mx.Unlock()
}

func (s *DataStore) updateVerC() {
	conf := s.GetConfig().Clone()
	conf.VersionC = genVersion()
	s.SetConfig(conf)
}

// User
func (s *DataStore) GetUserByAcc(acc string) *User {
	su := s.getShadowUser(acc)
	if su != nil {
		return su.Clone()
	}
	return s.User.GetByAcc(acc)
}
func (s *DataStore) GetUserByUID(uid UserID) *User {
	if uint64(uid) == 0 {
		s.mx.RLock()
		defer s.mx.RUnlock()
		for _, u := range s.ssUser {
			return u.Clone()
		}
	}
	return s.User.GetByID(uid)
}
func (s *DataStore) DelUserByUID(uid UserID) error {
	defer s.FlagDirty()
	return s.User.Del(uid)
}
func (s *DataStore) AddUser(u *User) (UserID, error) { // auto set UID
	defer s.FlagDirty()
	return s.User.Add(u)
}
func (s *DataStore) UpdateUser(u *User) error { // need exist
	defer s.FlagDirty()
	return s.User.Set(u)
}
func (s *DataStore) ListUser() []*User {
	return s.User.GetAll()
}


func (s *DataStore) AddShadowUser(acc string, pwd string) {
	u := &User{
		ID: 0,
		Acc: acc,
		Name: "shadow super user",
		Note: "should not use this in production",
		Super: true,
	}
	u.SetPasswd(pwd)

	s.mx.Lock()
	s.ssUser[acc] = u
	s.mx.Unlock()
}

func (s *DataStore) getShadowUser(acc string) *User {
	s.mx.RLock()
	defer s.mx.RUnlock()
	u, ok := s.ssUser[acc]
	if !ok {
		return nil
	}
	return u
}

// Attachment
func (s *DataStore) GetAttachByToken(token string) *Attachment { // for download link
	return s.Attach.GetByToken(token)
}
func (s *DataStore) GetAttachByAID(aid AttachID) *Attachment {
	return s.Attach.GetByID(aid)
}
func (s *DataStore) DelAttachByAID(aid AttachID) error { // TODO: also delete file
	defer s.FlagDirty()
	return s.Attach.Del(aid)
}
func (s *DataStore) UpdateAttach(attach *Attachment) error { // need exist, AID & Token should not change
	defer s.FlagDirty()
	return s.Attach.Set(attach)
}
func (s *DataStore) AddAttach(attach *Attachment) (AttachID, error) { // auto set AID & token
	defer s.FlagDirty()
	return s.Attach.Add(attach)
}
func (s *DataStore) ListAttach() []*Attachment {
	return s.Attach.GetWeb()
}

// Hook
func (s *DataStore) GetHookByToken(token string) *HookConfig { // for download
	return s.Hook.GetByToken(token)
}
func (s *DataStore) GetHookByAuthToken(authToken string) *HookConfig { // for data update
	return s.Hook.GetByAuthToken(authToken)
}
func (s *DataStore) GetHookByID(hid HookID) *HookConfig {
	return s.Hook.GetByID(hid)
}
func (s *DataStore) DelHookByID(hid HookID) error {
	defer s.FlagDirty()
	return s.Hook.Del(hid)
}
func (s *DataStore) AddHook(hk *HookConfig) (HookID, error) { // auto set HID & token & AuthToken
	defer s.FlagDirty()
	return s.Hook.Add(hk)
}
func (s *DataStore) UpdateHookConfig(hk *HookConfig) error { // only update config
	defer s.FlagDirty()
	return s.Hook.SetConfig(hk)
}
func (s *DataStore) UpdateHook(hk *HookConfig) error {
	defer s.FlagDirty()
	return s.Hook.Set(hk)
}
func (s *DataStore) ListHook() []*HookConfig { // return copy & clean up
	return s.Hook.GetWeb()
}

// Layer
func (s *DataStore) GetLayerByID(id LayerID) *LayerGroup {
	return s.Layer.GetByID(id)
}
func (s *DataStore) DelLayerByID(id LayerID) error {
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Layer.Del(id)
}
func (s *DataStore) AddLayer(layer *LayerGroup) (LayerID, error) { // auto set LyID
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Layer.Add(layer)
}
func (s *DataStore) UpdateLayer(layer *LayerGroup) error { // need exist
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Layer.Set(layer)
}
func (s *DataStore) GetAllLayer() []*LayerGroup {
	return s.Layer.GetAll()
}
func (s *DataStore) OrderLayer(ids []LayerID) error {
	defer s.FlagDirty()
	defer s.updateVerC()
	s.Layer.Order(ids)
	return nil
}


// BaseMap
func (s *DataStore) GetMapByID(id MapID) *BaseMap {
	return s.Map.GetByID(id)
}
func (s *DataStore) DelMapByID(id MapID) error {
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Map.Del(id)
}
func (s *DataStore) AddMap(m *BaseMap) (MapID, error) { // auto set MID
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Map.Add(m)
}
func (s *DataStore) UpdateMap(m *BaseMap) error { // need exist
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Map.Set(m)
}
func (s *DataStore) GetAllMap() []*BaseMap {
	return s.Map.GetAll()
}
func (s *DataStore) OrderMap(ids []MapID) error {
	defer s.FlagDirty()
	defer s.updateVerC()
	s.Map.Order(ids)
	return nil
}

// Link
func (s *DataStore) GetLinkByID(id LinkID) *Link {
	return s.Link.GetByID(id)
}
func (s *DataStore) DelLinkByID(id LinkID) error{
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Link.Del(id)
}
func (s *DataStore) AddLink(link *Link) (LinkID, error) { // auto set LkID
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Link.Add(link)
}
func (s *DataStore) UpdateLink(link *Link) error { // need exist
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Link.Set(link)
}
func (s *DataStore) GetAllLink() []*Link {
	return s.Link.GetAll()
}
func (s *DataStore) OrderLink(ids []*LinkOrder_S) error {
	// try fix indent level
	if len(ids) > 0 {
		var up = ids[0]
		var baseLv = up.LV
		for _, it := range ids {
			it.LV -= baseLv
			if it.LV < 0 {
				it.LV = 0
			}

			if it.LV - up.LV > 1 {
				it.LV = up.LV + 1
			}

			up = it
		}
	}

	defer s.FlagDirty()
	defer s.updateVerC()
	s.Link.Order(ids)
	return nil
}

// Tab
func (s *DataStore) GetTabByID(id TabID) *TabData {
	return s.Tab.GetByID(id)
}
func (s *DataStore) DelTabByID(id TabID) error {
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Tab.Del(id)
}
func (s *DataStore) AddTab(tab *TabData) (TabID, error) { // auto set TabID
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Tab.Add(tab)
}
func (s *DataStore) UpdateTab(tab *TabData) error { // need exist
	defer s.FlagDirty()
	defer s.updateVerC()
	return s.Tab.Set(tab)
}
func (s *DataStore) GetAllTab() []*TabData {
	return s.Tab.GetAll()
}
func (s *DataStore) OrderTab(ids []TabID) error {
	defer s.FlagDirty()
	defer s.updateVerC()
	s.Tab.Order(ids)
	return nil
}

