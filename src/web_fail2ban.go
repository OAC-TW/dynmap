package webmap

import (
	"sync"
	"time"
	"math/rand"
)

var (
	_window_time = 10 * 60 * time.Second // 10 min
)

// TODO: using Bloom Filter or Cuckoo Filter?
type countPool struct {
	mx  sync.RWMutex

	a   map[string]uint8
	swtA time.Time
	rstA time.Time

	b   map[string]uint8
	swtB time.Time
	rstB time.Time
}

func NewCountPool() *countPool {
	t0 := now()
	t1 := t0.Add(_window_time / 2)
	p := &countPool{
		a: make(map[string]uint8),
		b: make(map[string]uint8),
		swtA: t0,
		rstA: t0.Add(_window_time),
		swtB: t1,
		rstB: t1.Add(_window_time),
	}
	return p
}

func (p *countPool) Insert(key string) {
	p.mx.Lock()
	defer p.mx.Unlock()

	now := now()
	if now.After(p.rstA) { // rest A
		p.a = make(map[string]uint8)
		p.a[key] += 1

		rng := time.Duration(rand.Intn(6)) * 60 * time.Second // 0~5 min
		p.rstA = now.Add(_window_time + rng)
		p.swtA = now
		p.rstB = p.rstB.Add(rng) // extend B
	}
	if now.After(p.swtA) { // set to A
		p.a[key] += 1
	}

	if now.After(p.rstB) { // rest B
		p.b = make(map[string]uint8)
		p.b[key] += 1

		rng := time.Duration(rand.Intn(6)) * 60 * time.Second // 0~5 min
		p.rstB = now.Add(_window_time + rng)
		p.swtB = now
		p.rstA = p.rstA.Add(rng) // extend A
	}
	if now.After(p.swtB) { // set to B
		p.b[key] += 1
	}
}

func (p *countPool) Lookup(key string) uint8 {
	p.mx.RLock()
	defer p.mx.RUnlock()

	if p.swtB.After(p.swtA) { // if B is new
		return p.a[key]
	} else {
		return p.b[key]
	}
}

type Fail2Ban struct {
	ip   *countPool // IP -> count
	acc  *countPool // acc -> count
//	hash *countPool // hash(all) -> count
}

func (f2b *Fail2Ban) IsBanIP(ip string, limit int) bool {
	count := f2b.ip.Lookup(ip)
	if int(count) >= limit {
		return true
	}
	return false
}

func (f2b *Fail2Ban) IsBanAcc(acc string, limit int) bool {
	count := f2b.acc.Lookup(acc)
	if int(count) >= limit {
		return true
	}
	return false
}

func (f2b *Fail2Ban) AddFail(ip string, acc string) {
	f2b.ip.Insert(ip)
	f2b.acc.Insert(acc)
}

func NewFail2Ban() *Fail2Ban {
	f2b := &Fail2Ban{
		ip: NewCountPool(),
		acc: NewCountPool(),
	}
	return f2b
}

