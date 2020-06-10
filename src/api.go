package webmap

import (
	"errors"
)

var (
	ErrAccExist = errors.New("Login account exist!")
	ErrItemExist = errors.New("item exist!")
	ErrNotExist = errors.New("item not exist")
	ErrTokenGen = errors.New("generate token fail")
)

type API interface {
	Open(dbPath string) error
	Close() error
	Flush() error

	GetPubLayer() []*LayerGroup
	GetPubMap() []*BaseMap
	GetPubLink() []*Link
	GetPubTab() []*TabData

	GetPageView() uint64
	AddAndGetPageView() uint64
	GetUserVisit() uint64
	AddAndGetUserVisit() uint64

	GetConfig() *TmplIndex
	SetConfig(conf *TmplIndex)

	// for management

	// User
	GetUserByAcc(acc string) *User
	GetUserByUID(uid UserID) *User
	DelUserByUID(uid UserID) error
	AddUser(u *User) (UserID, error) // auto set UID
	UpdateUser(u *User) error // need exist
	ListUser() []*User // return copy & clean up
	AddShadowUser(acc string, pwd string) // temporary super user, not save into db

	// Attachment
	GetAttachByToken(token string) *Attachment // for download link
	//GetAttachByAID(aid AttachID) *Attachment
	DelAttachByAID(aid AttachID) error
	UpdateAttach(attach *Attachment) error
	AddAttach(attach *Attachment) (AttachID, error) // auto set AID & token
	ListAttach() []*Attachment // return copy & clean up

	// Hook
	GetHookByToken(token string) *HookConfig // for download
	GetHookByAuthToken(authToken string) *HookConfig // for data update
	GetHookByID(hid HookID) *HookConfig
	DelHookByID(hid HookID) error
	AddHook(hk *HookConfig) (HookID, error) // auto set HID & token & AuthToken
	UpdateHookConfig(hk *HookConfig) error // only update config
	UpdateHook(hk *HookConfig) error
	ListHook() []*HookConfig // return copy & clean up

	// Layer
	GetLayerByID(id LayerID) *LayerGroup
	DelLayerByID(id LayerID) error
	//HideLayerByID(id LayerID) error // use UpdateLayer
	AddLayer(layer *LayerGroup) (LayerID, error) // auto set LyID
	UpdateLayer(layer *LayerGroup) error // need exist
	GetAllLayer() []*LayerGroup // return copy
	OrderLayer(ids []LayerID) error

	// BaseMap
	GetMapByID(id MapID) *BaseMap
	DelMapByID(id MapID) error
	//HideMapByID(id MapID) error
	AddMap(m *BaseMap) (MapID, error) // auto set MID
	UpdateMap(m *BaseMap) error // need exist
	GetAllMap() []*BaseMap
	OrderMap(ids []MapID) error

	// Link
	GetLinkByID(id LinkID) *Link
	DelLinkByID(id LinkID) error
	//HideLinkByID(id LinkID) error
	AddLink(link *Link) (LinkID, error) // auto set LkID
	UpdateLink(link *Link) error // need exist
	GetAllLink() []*Link
	OrderLink(ids []*LinkOrder_S) error

	// Tab
	GetTabByID(id TabID) *TabData
	DelTabByID(id TabID) error
	AddTab(tab *TabData) (TabID, error) // auto set TabID
	UpdateTab(tab *TabData) error // need exist
	GetAllTab() []*TabData
	OrderTab(ids []TabID) error
}

