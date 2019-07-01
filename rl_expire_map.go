package expiremap

import (
	"github.com/nursik/go-ordered-set"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type rlVal struct {
	ttl int64
	key interface{}
	value interface{}
}

const rlExpireInterval = 100 * time.Millisecond
const pageBits = 10
const pageSize = 1<<pageBits
type page struct {
	size int
	values [pageSize]rlVal
}

type pages struct {
	pages [1000000]*page
}

func (ps *pages) put(index uint64, v rlVal) {
	bucket := index >> pageBits
	if ps.pages[bucket] == nil {
		ps.pages[bucket] = &page{}
	}
	page := ps.pages[bucket]
	page.values[index&(1<<pageBits - 1)] = v
	page.size++
}

func (ps *pages) remove(index uint64) {
	bucket := index >> pageBits
	page := ps.pages[bucket]
	page.size--
	if page.size == 0 {
		ps.pages[bucket] = nil
	}
}

func (ps *pages) get(index uint64) rlVal {
	bucket := index >> pageBits
	return ps.pages[bucket].values[index&(1<<pageBits - 1)]
}

type rlExpireMap struct {
	keys map[interface{}]uint64
	values *pages
	indices orderedset.OrderedSet
	sync.RWMutex
	stopped int64
	curtime int64
}

func (rl *rlExpireMap) Expire(key interface{}, due time.Time) bool {
	ttl := due.UnixNano()
	if ttl <= rl.Curtime() {
		rl.Delete(key)
		return false
	}
	rl.Lock()
	if rl.Stopped() {
		rl.Unlock()
		return false
	}
	id, ok := rl.keys[key]
	if ok == false {
		rl.Unlock()
		return false
	}
	rlv := rl.values.get(id)
	if rlv.ttl <= rl.Curtime() {
		rl.del(key, id)
		rl.Unlock()
		return false
	}
	rlv.ttl = ttl
	rl.values.put(id, rlv)
	rl.Unlock()
	return true
}

func (rl *rlExpireMap) Get(key interface{}) (interface{}, bool) {
	rl.RLock()
	if rl.Stopped() {
		rl.RUnlock()
		return nil, false
	}
	id, ok := rl.keys[key]
	if ok == false {
		rl.RUnlock()
		return nil, false
	}
	rlv := rl.values.get(id)
	if rlv.ttl > rl.Curtime() {
		rl.RUnlock()
		return rlv.value, true
	}
	rl.RUnlock()
	rl.Lock()
	if rl.Stopped() {
		rl.Unlock()
		return nil, false
	}

	id, ok = rl.keys[key]
	if ok == false {
		rl.Unlock()
		return nil, false
	}

	rlv = rl.values.get(id)
	if rlv.ttl > rl.Curtime() {
		rl.Unlock()
		return rlv.value, true
	} else {
		rl.del(key, id)
		rl.Unlock()
		return nil, false
	}
}

func (rl *rlExpireMap) Delete(key interface{}) {
	rl.Lock()
	if rl.Stopped() {
		rl.Unlock()
		return
	}
	id, ok := rl.keys[key]
	if ok == false {
		rl.Unlock()
		return
	}
	rl.del(key, id)
	rl.Unlock()
}

func (rl *rlExpireMap) Close() {
	rl.Lock()
	if rl.Stopped() == false {
		atomic.StoreInt64(&rl.stopped, 1)
		rl.keys = nil
		rl.values = nil
		rl.indices = nil
	}
	rl.Unlock()
}

func (rl *rlExpireMap) SetEx(key interface{}, value interface{}, due time.Time) {
	ttl := due.UnixNano()
	if ttl <= rl.Curtime() {
		return
	}
	rl.Lock()
	if rl.Stopped() {
		rl.Unlock()
		return
	}

	if id, ok := rl.keys[key]; ok {
		rl.values.put(id, rlVal{
			key: key,
			value: value,
			ttl: ttl,
		})
	} else {
		id := rl.indices.LowestUnused()
		rl.indices.Insert(id)
		rl.keys[key] = id
		rl.values.put(id, rlVal{
			key:   key,
			value: value,
			ttl:   ttl,
		})
	}
	rl.Unlock()
}

func (rl *rlExpireMap) GetAll() []KeyValue {
	rl.RLock()
	if rl.Stopped() {
		rl.Unlock()
		return nil
	}
	sz := rl.indices.Size()
	ans := make([]KeyValue, 0, sz)
	curtime := rl.Curtime()
	for i := 0; i < sz; i++ {
		id := rl.indices.Kth(i)
		rlv := rl.values.get(id)
		if rlv.ttl > curtime {
			ans = append(ans, KeyValue{key: rlv.key, value: rlv.value})
		}
	}
	rl.RUnlock()
	return ans
}

func (rl *rlExpireMap) Size() int {
	rl.RLock()
	sz := len(rl.keys)
	rl.RUnlock()
	return sz
}

func (rl *rlExpireMap) Stopped() bool {
	return atomic.LoadInt64(&rl.stopped) == 1
}

func (rl *rlExpireMap) Curtime() int64 {
	return atomic.LoadInt64(&rl.curtime)
}

func (rl *rlExpireMap) del(key interface{}, id uint64) {
	delete(rl.keys, key)
	rl.values.remove(id)
	rl.indices.Remove(id)
}

func (rl *rlExpireMap) randomExpire() bool {
	const totalChecks = 20
	if rl.Stopped() {
		return false
	}

	if sz := rl.indices.Size(); sz <= totalChecks * 5 {
		for i := sz - 1; i >= 0; i-- {
			id := rl.indices.Kth(i)
			rlval := rl.values.get(id)
			if rlval.ttl <= rl.Curtime() {
				rl.del(rlval.key, id)
			}
		}
		return false
	}

	expiredFound := 0

	for i := 0; i < totalChecks; i++ {
		sz := rl.indices.Size()
		id := rl.indices.Kth(rand.Intn(sz))
		rlval := rl.values.get(id)
		if rlval.ttl <= rl.Curtime() {
			rl.del(rlval.key, id)
		}
	}

	if expiredFound * 4 >= totalChecks {
		return true
	}
	return false
}

func (rl *rlExpireMap) rotateExpire(kth *int) {
	const totalChecks = 20
	if rl.Stopped() {
		return
	}
	sz := rl.indices.Size()
	if sz == 0 {
		return
	}
	if *kth >= sz || *kth <= 0 {
		*kth = sz - 1
	}
	for i := 0; i < totalChecks; i++ {
		id := rl.indices.Kth(*kth)
		rlval := rl.values.get(id)
		if rlval.ttl <= rl.Curtime() {
			rl.del(rlval.key, id)
		}
		*kth--
		if *kth < 0 {
			break
		}
	}
}

func (rl *rlExpireMap) start() {
	go func(){
		for {
			if rl.Stopped() {
				break
			}
			atomic.StoreInt64(&rl.curtime, time.Now().UnixNano())
			time.Sleep(timeResolution)
		}
	}()

	go func() {
		kth := 0
		for {
			if rl.Stopped() {
				break
			}
			start := time.Now()
			for i := 0; i < 10; i++ {
				rl.Lock()
				if !rl.randomExpire() {
					rl.Unlock()
					break
				}
				rl.Unlock()
			}
			rl.Lock()
			rl.rotateExpire(&kth)
			rl.Unlock()
			diff := time.Since(start)
			time.Sleep(rlExpireInterval - diff)
		}
	}()
}

func NewRLExpireMap() ExpireMap {
	rl := &rlExpireMap{
		keys: make(map[interface{}]uint64),
		indices: orderedset.NewTreeSet(),
		values: &pages{},
		curtime: time.Now().UnixNano(),
	}
	rl.start()
	return rl
}
