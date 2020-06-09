package webmap

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	_SESSION_TTL = 15 * 60 * time.Second // 15 min

	_low_precise_time  atomic.Value // time.Time
)

// low precise, should enough for session
func now() time.Time {
	t, ok := _low_precise_time.Load().(time.Time)
	if !ok {
		return time.Now()
	}
	return t
}

func init() {
	_low_precise_time.Store(time.Now())
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case now := <-ticker.C:
				_low_precise_time.Store(now)
			}
		}
	}()
}

type SessionData struct {
	mx     sync.RWMutex
	ttl    time.Time
	lst    map[interface{}]interface{}
}

func (sd *SessionData) IsTimeout() bool {
	sd.mx.RLock()
	defer sd.mx.RUnlock()
	return now().After(sd.ttl)
}

func (sd *SessionData) Renew() {
	sd.mx.Lock()
	sd.ttl = now().Add(_SESSION_TTL)
	sd.mx.Unlock()
}

func (sd *SessionData) Get(k interface{}) (interface{}, bool) {
	sd.mx.RLock()
	v, ok := sd.lst[k]
	sd.mx.RUnlock()
	return v, ok
}

func (sd *SessionData) Set(k interface{}, v interface{}) {
	sd.mx.Lock()
	sd.lst[k] = v
	sd.mx.Unlock()
}

func NewSessionData() *SessionData {
	sd := &SessionData{
		ttl: now().Add(_SESSION_TTL),
		lst: make(map[interface{}]interface{}),
	}
	return sd
}

type Session struct {
	die     chan struct{}
	mx      sync.RWMutex
	cookie  map[string]*SessionData
}

func (ss *Session) Len() int {
	ss.mx.RLock()
	sz := len(ss.cookie)
	ss.mx.RUnlock()
	return sz
}

func (ss *Session) GetOrRenewSession(token string) *SessionData {
	ss.mx.RLock()
	sd, ok := ss.cookie[token]
	ss.mx.RUnlock()
	if !ok {
		return nil
	}
	if sd.IsTimeout() {
		ss.mx.Lock()
		delete(ss.cookie, token)
		ss.mx.Unlock()
		return nil
	}
	sd.Renew()
	return sd
}

func (ss *Session) Destroy(token string) {
	ss.mx.RLock()
	_, ok := ss.cookie[token]
	ss.mx.RUnlock()
	if ok {
		ss.mx.Lock()
		delete(ss.cookie, token)
		ss.mx.Unlock()
	}
	return
}

func (ss *Session) New(token string) *SessionData {
	sd := NewSessionData()
	ss.mx.Lock()
	sd0, ok := ss.cookie[token]
	if ok {
		ss.mx.Unlock()
		return sd0
	}
	ss.cookie[token] = sd
	ss.mx.Unlock()
	return sd
}

func (ss *Session) NewToken() (string, *SessionData) {
	sd := NewSessionData()
	token := genToken()

	fail := true
	ss.mx.Lock()
	for i:=0; i<10000; i++ {
		_, ok := ss.cookie[token]
		if !ok {
			ss.cookie[token] = sd
			fail = false
			break
		}
		token = genToken()
	}
	ss.mx.Unlock()

	if fail {
		return "", nil
	}
	return token, sd
}

func (ss *Session) clean() {
	ss.mx.RLock()
	rmLst := make([]string, 0, len(ss.cookie))
	for k, sd := range ss.cookie {
		if sd.IsTimeout() {
			rmLst = append(rmLst, k)
		}
	}
	ss.mx.RUnlock()

	if len(rmLst) > 0 {
		ss.mx.Lock()
		for _, token := range rmLst {
			delete(ss.cookie, token)
		}
		ss.mx.Unlock()
	}
}

func (ss *Session) cleaner() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ss.die:
			return
		case <-ticker.C:
			ss.clean()
		}
	}
}

func (ss *Session) Close() {
	select {
	case <-ss.die:
	default:
		close(ss.die)
	}
}

func NewSession() *Session {
	sess := &Session{
		cookie: make(map[string]*SessionData),
		die: make(chan struct{}),
	}
	go sess.cleaner()
	return sess
}

