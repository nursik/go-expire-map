// Copyright (c) 2019 Nursultan Zarlyk. All rights reserved.
// Use of this source code is governed by the MIT License that can be found in the LICENSE file.

// Package expiremap provides a thread-safe map with expiring keys.
// You must pay attention for these facts:
// 		1) Current implementation may hold up to 1 billion keys
// 		2) All methods use Curtime() and it differs from time.Now() - Curtime() lags behind
// 		time.Now() on average 0.5 ms and maximum 5 ms (see tests). So expire accuracy error
// 		is around 1 ms
// 		3) After creating a new map (calling New()), two goroutines are created - one
// 		for updating curtime and another for deletion of expired keys. They exist until
// 		Close() method is called
//		4) There are active and passive expirations. Active expiration is done during Get(),
//		and SetTTL() calls. Passive expiration happens in background and is done by goroutine
// 		4) Passive expiration occurs every 100ms. It is done in two steps - first step is
// 		inspired by algorithm used in Redis and second step is sequential expiration
// 		5) It is guaranteed by sequential expiration, that no expired key will live more than
// 		map.Size() / 200 seconds
//
// First step's (or random expire) algorithm is following:
// 		1) Check the size of the map. If it is less than 100, just iterate over all keys
// 		and stop algorithm
// 		2) Check 20 random keys. Remove all expired keys. If there were at least 5 deletions,
// 		do the step 2 again (step 2 is done maximum 10 times)
// Second step's (or rotate expire) algorithm is following:
// 		1) Load to X a key on which we stopped on the previous call. If on previous call
// 		we hit the bottom of the map, load top key of the map
// 		2) Start from the key X and from that key expire 20 consecutive keys or stop if
// 		we hit a bottom of the map
//
// It means that at maximum 2200 expires per second may occur (not counting active expiration).
// If you have a lot of insertions with unique keys, but you rarely call methods Get and SetTTL
// on these keys, your map will grow faster than expiration rate and you may hit 1 billion keys
// limit.
package expiremap

import (
	"github.com/nursik/go-ordered-set"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// time interval for updating curtime variable
const timeResolution = time.Millisecond

// time interval for calling randomExpire and rotateExpire methods
const expireInterval = 100 * time.Millisecond

// KeyValue is used only for GetAll method
type KeyValue struct {
	Key   interface{}
	Value interface{}
}

type item struct {
	// ttl is unix nanoseconds. Instance should live up to this time. Probably, should be renamed to due
	ttl   int64
	key   interface{}
	value interface{}
}

const pageBitSize = 10
const pageSize = 1 << pageBitSize
const pagesPerMap = 1000000

type page struct {
	size   int
	values [pageSize]item
}

type pages struct {
	pages [pagesPerMap]*page
}

func (ps *pages) put(index uint64, v item) {
	bucket := index >> pageBitSize
	if ps.pages[bucket] == nil {
		ps.pages[bucket] = &page{}
	}
	page := ps.pages[bucket]
	page.values[index&(1<<pageBitSize-1)] = v
	page.size++
}

func (ps *pages) remove(index uint64) {
	bucket := index >> pageBitSize
	page := ps.pages[bucket]
	page.size--
	if page.size == 0 {
		ps.pages[bucket] = nil
	} else {
		page.values[index&(1<<pageBitSize-1)].value = nil
	}
}

func (ps *pages) get(index uint64) item {
	bucket := index >> pageBitSize
	return ps.pages[bucket].values[index&(1<<pageBitSize-1)]
}

// ExpireMap stores keys and corresponding values and TTLs.
type ExpireMap struct {
	keys    map[interface{}]uint64
	values  *pages
	indices orderedset.OrderedSet
	mutex   sync.RWMutex
	stopped int64
	curtime int64
}

// SetTTL updates ttl for the given key. If ttl was successfully updated,
// it returns value and "true". It happens, if and only if key presents
// in the map and "ttl" variable is greater than timeResolution. In any other
// case it returns nil and "false". Also, if "ttl" variable is less than
// timeResolution, it just removes a key.
func (m *ExpireMap) SetTTL(key interface{}, ttl time.Duration) (interface{}, bool) {
	if ttl <= timeResolution {
		m.Delete(key)
		return nil, false
	}
	due := int64(ttl/time.Nanosecond) + m.Curtime()

	m.mutex.Lock()
	if m.Stopped() {
		m.mutex.Unlock()
		return nil, false
	}
	id, ok := m.keys[key]
	if ok == false {
		m.mutex.Unlock()
		return nil, false
	}
	v := m.values.get(id)
	if v.ttl <= m.Curtime() {
		m.del(key, id)
		m.mutex.Unlock()
		return nil, false
	}
	v.ttl = due
	m.values.put(id, v)
	m.mutex.Unlock()
	return v.value, true
}

// Get returns value for the given key. If map does not contain
// such key or key is expired, it returns nil and "false". If key is
// expired it waits for write lock, checks a ttl again (as during wait of
// write lock, value and ttl could be updated) and if it is still expired,
// removes the given key (otherwise it returns a value and "true"). So basically,
// with increase of the number of hits to expired key, performance of Get method
// lowers.
func (m *ExpireMap) Get(key interface{}) (interface{}, bool) {
	m.mutex.RLock()
	if m.Stopped() {
		m.mutex.RUnlock()
		return nil, false
	}
	id, ok := m.keys[key]
	if ok == false {
		m.mutex.RUnlock()
		return nil, false
	}
	v := m.values.get(id)
	if v.ttl > m.Curtime() {
		m.mutex.RUnlock()
		return v.value, true
	}
	m.mutex.RUnlock()
	m.mutex.Lock()
	if m.Stopped() {
		m.mutex.Unlock()
		return nil, false
	}

	id, ok = m.keys[key]
	if ok == false {
		m.mutex.Unlock()
		return nil, false
	}

	v = m.values.get(id)
	if v.ttl > m.Curtime() {
		m.mutex.Unlock()
		return v.value, true
	}

	m.del(key, id)
	m.mutex.Unlock()
	return nil, false
}

// GetTTL returns ttl for the given key as Unix nanoseconds, if it is not expired
// and exists in the map. Otherwise, it returns 0.
func (m *ExpireMap) GetTTL(key interface{}) int64 {
	m.mutex.RLock()
	if m.Stopped() {
		m.mutex.RUnlock()
		return 0
	}
	id, ok := m.keys[key]
	if ok == false {
		m.mutex.RUnlock()
		return 0
	}
	v := m.values.get(id)
	if cur := m.Curtime(); v.ttl > cur {
		ttl := v.ttl - cur
		m.mutex.RUnlock()
		return ttl
	}
	m.mutex.RUnlock()
	return 0
}

// Delete removes key from the map.
func (m *ExpireMap) Delete(key interface{}) {
	m.mutex.Lock()
	if m.Stopped() {
		m.mutex.Unlock()
		return
	}
	if id, ok := m.keys[key]; ok {
		m.del(key, id)
	}
	m.mutex.Unlock()
}

// Close stops internal goroutines and removes all internal structures.
func (m *ExpireMap) Close() {
	m.mutex.Lock()
	if m.Stopped() == false {
		atomic.StoreInt64(&m.stopped, 1)
		m.keys = nil
		m.values = nil
		m.indices = nil
	}
	m.mutex.Unlock()
}

// Set sets or updates value and ttl for the given key
func (m *ExpireMap) Set(key interface{}, value interface{}, ttl time.Duration) {
	if ttl < timeResolution {
		return
	}
	due := int64(ttl/time.Nanosecond) + m.Curtime()
	m.mutex.Lock()
	if m.Stopped() {
		m.mutex.Unlock()
		return
	}

	id, ok := m.keys[key]
	if !ok {
		id = m.indices.LowestUnused()
		m.indices.Insert(id)
		m.keys[key] = id
	}
	m.values.put(id, item{
		key:   key,
		value: value,
		ttl:   due,
	})
	m.mutex.Unlock()
}

// GetAll returns a slice of KeyValue. It guarantees that all
// keys are presented in the map and were not expired at the moment
// of method call.
func (m *ExpireMap) GetAll() []KeyValue {
	m.mutex.RLock()
	if m.Stopped() {
		m.mutex.RUnlock()
		return nil
	}
	sz := m.indices.Size()
	ans := make([]KeyValue, 0, sz)
	curtime := m.Curtime()
	for i := 0; i < sz; i++ {
		id := m.indices.Kth(i)
		v := m.values.get(id)
		if v.ttl > curtime {
			ans = append(ans, KeyValue{Key: v.key, Value: v.value})
		}
	}
	m.mutex.RUnlock()
	return ans
}

// Size returns a number of keys in the map, both expired and unexpired.
func (m *ExpireMap) Size() int {
	m.mutex.RLock()
	sz := len(m.keys)
	m.mutex.RUnlock()
	return sz
}

// Stopped indicates that map is stopped
func (m *ExpireMap) Stopped() bool {
	return atomic.LoadInt64(&m.stopped) == 1
}

// Curtime returns Unix nanoseconds. You may use it instead of calling time.Now().UnixNano().
// It lags behind time.Now() and on average difference is less than timeResolution,
// which is 1 millisecond, but sometimes difference may be up to 5 milliseconds.
func (m *ExpireMap) Curtime() int64 {
	return atomic.LoadInt64(&m.curtime)
}

// del is helper method to delete key and associated id from the map
func (m *ExpireMap) del(key interface{}, id uint64) {
	delete(m.keys, key)
	m.values.remove(id)
	m.indices.Remove(id)
}

// randomExpire randomly gets keys and checks for expiration.
// The common logic was inspired by Redis.
func (m *ExpireMap) randomExpire() bool {
	const totalChecks = 20
	const bruteForceThreshold = 100
	if m.Stopped() {
		return false
	}
	// Because the number of keys is small, just iterate over all keys
	if sz := m.indices.Size(); sz <= bruteForceThreshold {
		for i := sz - 1; i >= 0; i-- {
			id := m.indices.Kth(i)
			v := m.values.get(id)
			if v.ttl <= m.Curtime() {
				m.del(v.key, id)
			}
		}
		return false
	}

	expiredFound := 0

	for i := 0; i < totalChecks; i++ {
		sz := m.indices.Size()
		id := m.indices.Kth(rand.Intn(sz))
		v := m.values.get(id)
		if v.ttl <= m.Curtime() {
			m.del(v.key, id)
			expiredFound++
		}
	}

	if expiredFound*4 >= totalChecks {
		return true
	}
	return false
}

// rotateExpire checks keys sequentially for expiration.
// Some keys may live too long, because randomExpire cannot hit them, and
// that's why this method was written. Basically, it iterates over 20 keys
// and checks them. The passed variable is k-th key, which previously was
// checked.
func (m *ExpireMap) rotateExpire(kth int) int {
	const totalChecks = 20
	if m.Stopped() {
		return 0
	}
	sz := m.indices.Size()
	if sz == 0 {
		return 0
	}
	if kth >= sz || kth <= 0 {
		kth = sz - 1
	}
	for i := 0; i < totalChecks; i++ {
		id := m.indices.Kth(kth)
		v := m.values.get(id)
		if v.ttl <= m.Curtime() {
			m.del(v.key, id)
		}
		kth--
		if kth < 0 {
			break
		}
	}
	return kth
}

// start starts two goroutines - first for updating curtime variable and
// second for expiration of keys. for loops with time.Sleep are used instead
// of time tickers.
func (m *ExpireMap) start() {
	go func() {
		for {
			if m.Stopped() {
				break
			}
			atomic.StoreInt64(&m.curtime, time.Now().UnixNano())
			time.Sleep(timeResolution)
		}
	}()

	go func() {
		kth := 0
		for {
			if m.Stopped() {
				break
			}
			start := time.Now()
			for i := 0; i < 10; i++ {
				m.mutex.Lock()
				if !m.randomExpire() {
					m.mutex.Unlock()
					break
				}
				m.mutex.Unlock()
			}
			m.mutex.Lock()
			kth = m.rotateExpire(kth)
			m.mutex.Unlock()
			diff := time.Since(start)
			time.Sleep(expireInterval - diff)
		}
	}()
}

// New returns a new map.
func New() *ExpireMap {
	rl := &ExpireMap{
		keys:    make(map[interface{}]uint64),
		indices: orderedset.NewTreeSet(),
		values:  &pages{},
		curtime: time.Now().UnixNano(),
	}
	rl.start()
	return rl
}
