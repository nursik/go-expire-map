// Copyright (c) 2019 Nursultan Zarlyk. All rights reserved.
// Use of this source code is governed by the MIT License that can be found in the LICENSE file.

// Package provides a thread-safe map with expiring keys.
// You must pay attention for these facts:
// 		1) Current implementation may hold up to 1 billion keys
// 		2) All methods use Curtime() and it differs from time.Now() - average difference
// 		around 0.5 ms, max difference around 4 ms. So expire accuracy error is around 1 ms
// 		3) After creating a new map (calling New()), two goroutines are created - one
// 		for updating curtime and another for deletion of expired keys. They exist until
// 		Close() method is called
// 		4) Every 100ms passive expiration occurs. It is done in two steps - first step is
// 		inspired with algorithm used in Redis and second step is sequential expiration
// 		5) It is guaranteed by sequential expiration, that no key will live more than
// 		map.Size() / 200 seconds
// 		6) There is an active expiration. Any call of Get() and Expire() on expired keys,
// 		removes them. Maybe in the future, TTL() will remove expired key too
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


type KeyValue struct {
	Key interface{}
	Value interface{}
}

type item struct {
	ttl int64
	key interface{}
	value interface{}
}

const pageBitSize = 10
const pageSize = 1<< pageBitSize
const pagesPerMap = 1000000

type page struct {
	size int
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
	page.values[index&(1<<pageBitSize- 1)] = v
	page.size++
}

func (ps *pages) remove(index uint64) {
	bucket := index >> pageBitSize
	page := ps.pages[bucket]
	page.size--
	if page.size == 0 {
		ps.pages[bucket] = nil
	}
}

func (ps *pages) get(index uint64) item {
	bucket := index >> pageBitSize
	return ps.pages[bucket].values[index&(1<<pageBitSize- 1)]
}

type ExpireMap struct {
	keys map[interface{}]uint64
	values *pages
	indices orderedset.OrderedSet
	sync.RWMutex
	stopped int64
	curtime int64
}

/*
	Expire updates ttl for the given key. If ttl was successfully updated,
	it returns value and "true". It happens, if and only if key presents
	in the map and due variable is greater than curtime. In any other
	case it returns nil and "false". Also, if due variable is less than
	curtime, it just removes a key.
 */
func (emp *ExpireMap) Expire(key interface{}, due time.Time) (interface{}, bool) {
	ttl := due.UnixNano()
	if ttl <= emp.Curtime() {
		emp.Delete(key)
		return nil, false
	}
	emp.Lock()
	if emp.Stopped() {
		emp.Unlock()
		return nil, false
	}
	id, ok := emp.keys[key]
	if ok == false {
		emp.Unlock()
		return nil, false
	}
	v := emp.values.get(id)
	if v.ttl <= emp.Curtime() {
		emp.del(key, id)
		emp.Unlock()
		return nil, false
	}
	v.ttl = ttl
	emp.values.put(id, v)
	emp.Unlock()
	return v.value, true
}

/*
	Get returns value for the given key. If map does not contain
	such key or key is expired, it returns nil and "false". If key is
	expired it waits for write lock, checks a ttl again (as during wait of
	write lock, value and ttl could be updated) and if it is still expired,
	removes the given key (otherwise it returns a value and "true"). So basically,
	with increase of the number of hits to expired key, performance of Get method
	lowers.
 */
func (emp *ExpireMap) Get(key interface{}) (interface{}, bool) {
	emp.RLock()
	if emp.Stopped() {
		emp.RUnlock()
		return nil, false
	}
	id, ok := emp.keys[key]
	if ok == false {
		emp.RUnlock()
		return nil, false
	}
	v := emp.values.get(id)
	if v.ttl > emp.Curtime() {
		emp.RUnlock()
		return v.value, true
	}
	emp.RUnlock()
	emp.Lock()
	if emp.Stopped() {
		emp.Unlock()
		return nil, false
	}

	id, ok = emp.keys[key]
	if ok == false {
		emp.Unlock()
		return nil, false
	}

	v = emp.values.get(id)
	if v.ttl > emp.Curtime() {
		emp.Unlock()
		return v.value, true
	} else {
		emp.del(key, id)
		emp.Unlock()
		return nil, false
	}
}

// TTL returns ttl for the given key as Unix nanoseconds, if it is not expired
// and exists in the map. Otherwise, it returns 0.
func (emp *ExpireMap) TTL(key interface{}) int64 {
	emp.RLock()
	if emp.Stopped() {
		emp.RUnlock()
		return 0
	}
	id, ok := emp.keys[key]
	if ok == false {
		emp.RUnlock()
		return 0
	}
	v := emp.values.get(id)
	if v.ttl > emp.Curtime() {
		emp.RUnlock()
		return v.ttl
	}
	emp.RUnlock()
	return 0
}

/*
	Delete removes key from the map.
 */
func (emp *ExpireMap) Delete(key interface{}) {
	emp.Lock()
	if emp.Stopped() {
		emp.Unlock()
		return
	}
	id, ok := emp.keys[key]
	if ok == false {
		emp.Unlock()
		return
	}
	emp.del(key, id)
	emp.Unlock()
}

/*
	Close stops internal goroutines and removes all internal structures.
 */
func (emp *ExpireMap) Close() {
	emp.Lock()
	if emp.Stopped() == false {
		atomic.StoreInt64(&emp.stopped, 1)
		emp.keys = nil
		emp.values = nil
		emp.indices = nil
	}
	emp.Unlock()
}

/*
	SetEx sets or updates value and ttl for the given key
 */
func (emp *ExpireMap) SetEx(key interface{}, value interface{}, due time.Time) {
	ttl := due.UnixNano()
	if ttl <= emp.Curtime() {
		return
	}
	emp.Lock()
	if emp.Stopped() {
		emp.Unlock()
		return
	}

	if id, ok := emp.keys[key]; ok {
		emp.values.put(id, item{
			key: key,
			value: value,
			ttl: ttl,
		})
	} else {
		id := emp.indices.LowestUnused()
		emp.indices.Insert(id)
		emp.keys[key] = id
		emp.values.put(id, item{
			key:   key,
			value: value,
			ttl:   ttl,
		})
	}
	emp.Unlock()
}

/*
	GetAll returns a slice of KeyValue. It guarantees that all
	keys are presented in the map and were not expired at the moment
	of method call.
 */
func (emp *ExpireMap) GetAll() []KeyValue {
	emp.RLock()
	if emp.Stopped() {
		emp.RUnlock()
		return nil
	}
	sz := emp.indices.Size()
	ans := make([]KeyValue, 0, sz)
	curtime := emp.Curtime()
	for i := 0; i < sz; i++ {
		id := emp.indices.Kth(i)
		v := emp.values.get(id)
		if v.ttl > curtime {
			ans = append(ans, KeyValue{key: v.key, value: v.value})
		}
	}
	emp.RUnlock()
	return ans
}

// Size returns a number of keys in the map, both expired and unexpired.
func (emp *ExpireMap) Size() int {
	emp.RLock()
	sz := len(emp.keys)
	emp.RUnlock()
	return sz
}

// Stopped indicates if the map is stopped (not valid for future uses)
func (emp *ExpireMap) Stopped() bool {
	return atomic.LoadInt64(&emp.stopped) == 1
}

/*
	Curtime returns Unix nanoseconds. You may use it instead of calling time.Now().UnixNano().
	The average difference between the value and real time.Now is less than timeResolution,
	which is 1 millisecond, but sometimes difference may be up to 4 milliseconds.
 */
func (emp *ExpireMap) Curtime() int64 {
	return atomic.LoadInt64(&emp.curtime)
}

func (emp *ExpireMap) del(key interface{}, id uint64) {
	delete(emp.keys, key)
	emp.values.remove(id)
	emp.indices.Remove(id)
}

// randomExpire randomly gets keys and checks for expiration.
// The common logic was inspired by Redis.
func (emp *ExpireMap) randomExpire() bool {
	const totalChecks = 20
	const bruteForceThreshold = totalChecks * 5
	if emp.Stopped() {
		return false
	}
	// Because the number of keys is small, just iterate over all keys
	if sz := emp.indices.Size(); sz <= bruteForceThreshold {
		for i := sz - 1; i >= 0; i-- {
			id := emp.indices.Kth(i)
			v := emp.values.get(id)
			if v.ttl <= emp.Curtime() {
				emp.del(v.key, id)
			}
		}
		return false
	}

	expiredFound := 0

	for i := 0; i < totalChecks; i++ {
		sz := emp.indices.Size()
		id := emp.indices.Kth(rand.Intn(sz))
		v := emp.values.get(id)
		if v.ttl <= emp.Curtime() {
			emp.del(v.key, id)
		}
	}

	if expiredFound * 4 >= totalChecks {
		return true
	}
	return false
}

/*
	rotateExpire checks keys sequentially for expiration.
	Some keys may live too long, because randomExpire cannot hit them, and
	that's why this method was written. Basically, it iterates over 20 keys
	and checks them. The passed variable is k-th key, which previously was
	checked.
  */
func (emp *ExpireMap) rotateExpire(kth *int) {
	const totalChecks = 20
	if emp.Stopped() {
		return
	}
	sz := emp.indices.Size()
	if sz == 0 {
		return
	}
	if *kth >= sz || *kth <= 0 {
		*kth = sz - 1
	}
	for i := 0; i < totalChecks; i++ {
		id := emp.indices.Kth(*kth)
		v := emp.values.get(id)
		if v.ttl <= emp.Curtime() {
			emp.del(v.key, id)
		}
		*kth--
		if *kth < 0 {
			break
		}
	}
}

// start starts two goroutines - first for updating curtime variable and
// second for expiration of keys. for loops with time.Sleep are used instead
// of time tickers.
func (emp *ExpireMap) start() {
	go func(){
		for {
			if emp.Stopped() {
				break
			}
			atomic.StoreInt64(&emp.curtime, time.Now().UnixNano())
			time.Sleep(timeResolution)
		}
	}()

	go func() {
		kth := 0
		for {
			if emp.Stopped() {
				break
			}
			start := time.Now()
			for i := 0; i < 10; i++ {
				emp.Lock()
				if !emp.randomExpire() {
					emp.Unlock()
					break
				}
				emp.Unlock()
			}
			emp.Lock()
			emp.rotateExpire(&kth)
			emp.Unlock()
			diff := time.Since(start)
			time.Sleep(expireInterval - diff)
		}
	}()
}

// New returns a new map.
func New() *ExpireMap {
	rl := &ExpireMap{
		keys: make(map[interface{}]uint64),
		indices: orderedset.NewTreeSet(),
		values: &pages{},
		curtime: time.Now().UnixNano(),
	}
	rl.start()
	return rl
}
