// Copyright (c) 2024 Nursultan Zarlyk. All rights reserved.
// Use of this source code is governed by the MIT License that can be found in the LICENSE file.

// Package expiremap provides a thread-safe map with expiring keys.
// You must pay attention for these facts:
//  1. Current implementation may hold up to 1 billion keys
//  2. After creating a new map (calling New()), goroutine is created for deletion
//     expired keys. It exists until Close() method is called
//  3. There are active and passive expirations. Active expiration is done during Get(),
//     and SetTTL() calls. Passive expiration happens in background and is done by goroutine
//  4. Passive expiration occurs every 100ms. It is done in two steps - first step is
//     inspired by algorithm used in Redis and second step is sequential expiration
//  5. It is guaranteed by sequential expiration, that no expired key will live more than
//     map.Size() / 200 seconds
//
// First step's (or random expire) algorithm is following:
//  1. Check the size of the map. If it is less than 100, just iterate over all keys
//     and stop algorithm
//  2. Check 20 random keys. Remove all expired keys. If there were at least 5 deletions,
//     do the step 2 again (step 2 is done maximum 10 times)
//
// Second step's (or rotate expire) algorithm is following:
//  1. Load to X a key on which we stopped on the previous call. If on previous call
//     we hit the bottom of the map, load top key of the map
//  2. Start from the key X and from that key expire 20 consecutive keys or stop if
//     we hit a bottom of the map
//
// It means that at maximum 2200 expires per second may occur (not counting active expiration).
// If you have a lot of insertions with unique keys, but you rarely call methods Get and SetTTL
// on these keys, your map will grow faster than expiration rate and you may hit 1 billion keys
// limit.
package expiremap

import (
	"math/rand"
	"sync"
	"time"
)

// time interval for calling randomExpire and rotateExpire methods
const expireInterval = 100 * time.Millisecond

// EventType is used for notification channel
type EventType uint8

const (
	// Expire event is fired, when key is deleted due to expiration
	Expire EventType = 1 << iota
	// Delete event is fired, when key explicitly deleted using Delete
	// or SetTTL with non positive TTL
	Delete
	// Update event is fired, when TTL or value is updated
	Update
	// Set event is fired, when new key is inserted
	Set
	// AllEvents is a helper constant, which is the same as
	// Expire | Delete | Update | Set
	AllEvents = Expire | Delete | Update | Set
	// NoEvents is a helper constant, which is the same as 0 (no events)
	NoEvents = 0
)

// Event is used for notification channel
type Event struct {
	Key   interface{}
	Value interface{}
	// Time is when this event occurred (in Unix nanoseconds)
	Time int64
	// Due is when this key will expire (in Unix nanoseconds, 0 for expired)
	Due int64
	// Type of the Event. Equal to Expire, Delete, Update or Set.
	Type EventType
}

// KeyValue is used only for GetAll method
type KeyValue struct {
	Key   interface{}
	Value interface{}
}

type item struct {
	// due is unix nanoseconds. Instance should live up to this time
	due   int64
	key   interface{}
	value interface{}
}

// ExpireMap stores keys and corresponding values and TTLs.
type ExpireMap struct {
	keys    map[interface{}]int
	values  []item
	mutex   sync.RWMutex
	stopped bool
	now     func() int64
	c       chan<- Event
	events  EventType
}

// SetTTL updates ttl for the given key. If ttl was successfully updated,
// it returns value and "true". It happens, if and only if key presents
// in the map and "ttl" variable is greater than timeResolution. In any other
// case it returns nil and "false". Also, if "ttl" variable is non positive,
// it just removes a key.
func (m *ExpireMap) SetTTL(key interface{}, ttl time.Duration) (interface{}, bool) {
	if ttl <= 0 {
		m.Delete(key)
		return nil, false
	}
	curtime := m.now()
	due := int64(ttl/time.Nanosecond) + curtime

	m.mutex.Lock()
	if m.stopped {
		m.mutex.Unlock()
		return nil, false
	}
	id, ok := m.keys[key]
	if !ok {
		m.mutex.Unlock()
		return nil, false
	}
	v := m.values[id]
	if v.due <= m.now() {
		m.del(key, id, Expire)
		m.mutex.Unlock()
		return nil, false
	}

	if (m.events&Update) > 0 && m.c != nil {
		m.c <- Event{
			Key:   key,
			Value: v.value,
			Type:  Update,
			Time:  curtime,
			Due:   due,
		}
	}

	v.due = due
	m.values[id] = v
	m.mutex.Unlock()
	return v.value, true
}

// Get returns value for the given key. If map does not contain
// such key or key is expired, it returns nil and "false". If key is expired,
// then it waits for write lock, checks a ttl again (as during wait of
// write lock, value and ttl could be updated) and if it is still expired,
// removes the given key (otherwise it returns a value and "true").
func (m *ExpireMap) Get(key interface{}) (interface{}, bool) {
	m.mutex.RLock()
	if m.stopped {
		m.mutex.RUnlock()
		return nil, false
	}
	id, ok := m.keys[key]
	if !ok {
		m.mutex.RUnlock()
		return nil, false
	}
	v := m.values[id]
	if v.due > m.now() {
		m.mutex.RUnlock()
		return v.value, true
	}
	m.mutex.RUnlock()
	m.mutex.Lock()
	if m.stopped {
		m.mutex.Unlock()
		return nil, false
	}

	id, ok = m.keys[key]
	if !ok {
		m.mutex.Unlock()
		return nil, false
	}

	v = m.values[id]
	if v.due > m.now() {
		m.mutex.Unlock()
		return v.value, true
	}

	m.del(key, id, Expire)
	m.mutex.Unlock()
	return nil, false
}

// GetTTL returns time in nanoseconds, when key will die. If key is expired
// or does not exist in the map, it returns 0.
func (m *ExpireMap) GetTTL(key interface{}) int64 {
	m.mutex.RLock()
	if m.stopped {
		m.mutex.RUnlock()
		return 0
	}
	id, ok := m.keys[key]
	if !ok {
		m.mutex.RUnlock()
		return 0
	}
	v := m.values[id]
	if cur := m.now(); v.due > cur {
		ttl := v.due - cur
		m.mutex.RUnlock()
		return ttl
	}
	m.mutex.RUnlock()
	return 0
}

// Delete removes key from the map.
func (m *ExpireMap) Delete(key interface{}) {
	m.mutex.Lock()
	if m.stopped {
		m.mutex.Unlock()
		return
	}
	if id, ok := m.keys[key]; ok {
		m.del(key, id, Delete)
	}
	m.mutex.Unlock()
}

// Close stops goroutine and channel.
func (m *ExpireMap) Close() {
	m.mutex.Lock()
	if !m.stopped {
		m.stopped = true
		m.keys = nil
		m.values = nil
		if m.c != nil {
			close(m.c)
		}
		m.c = nil
	}
	m.mutex.Unlock()
}

// Set sets or updates value and ttl for the given key
func (m *ExpireMap) Set(key interface{}, value interface{}, ttl time.Duration) {
	curtime := m.now()
	due := int64(ttl/time.Nanosecond) + curtime
	m.mutex.Lock()
	if m.stopped {
		m.mutex.Unlock()
		return
	}

	t := Update
	id, ok := m.keys[key]
	if !ok {
		id = len(m.keys)
		m.keys[key] = id
		m.values = append(m.values, item{})
		t = Set
	}
	m.values[id] = item{
		key:   key,
		value: value,
		due:   due,
	}
	if (m.events&t) > 0 && m.c != nil {
		m.c <- Event{
			Key:   key,
			Value: value,
			Due:   due,
			Time:  curtime,
			Type:  t,
		}
	}
	m.mutex.Unlock()
}

// GetAll returns a slice of KeyValue.
func (m *ExpireMap) GetAll() []KeyValue {
	m.mutex.RLock()
	if m.stopped {
		m.mutex.RUnlock()
		return nil
	}
	var ans []KeyValue
	curtime := m.now()
	for _, v := range m.values {
		if v.due > curtime {
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

// Stopped indicates that map is stopped.
func (m *ExpireMap) Stopped() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.stopped
}

// Notify sets a channel and causes a map to send Event to a channel based on the given
// EventType. To get events X1, X2... pass X1|X2|...(for example, to get Update and Set
// events pass Update|Set). To receive all events use AllEvents constant (the same as
// Update|Set|Delete|Expire). To receive no events use NoEvents constant or set nil chan.
// When the map stops, no events are guaranteed to be sent to the channel.
// It is up to the user to close the channel.
func (m *ExpireMap) Notify(c chan<- Event, events EventType) {
	m.mutex.Lock()
	if m.stopped {
		m.mutex.Unlock()
		return
	}
	m.c = c
	m.events = events
	m.mutex.Unlock()
}

// del is helper method to delete key and associated id from the map
func (m *ExpireMap) del(key interface{}, id int, t EventType) {
	itemToDelete := m.values[id]

	if last := len(m.keys) - 1; id != last {
		lastItem := m.values[last]
		m.values[id] = lastItem
		m.keys[lastItem.key] = id
	}

	delete(m.keys, key)
	m.values[len(m.values)-1] = item{}
	m.values = m.values[:len(m.values)-1]

	if (m.events&t) > 0 && m.c != nil {
		m.c <- Event{
			Key:   key,
			Value: itemToDelete.value,
			Time:  m.now(),
			Type:  t,
		}
	}

}

// randomExpire randomly gets keys and checks for expiration.
// The common logic was inspired by Redis.
func (m *ExpireMap) randomExpire() bool {
	const totalChecks = 20
	const bruteForceThreshold = 100
	if m.stopped {
		return false
	}
	// Because the number of keys is small, just iterate over all keys
	if sz := len(m.keys); sz <= bruteForceThreshold {
		for _, id := range m.keys {
			v := m.values[id]
			if v.due <= m.now() {
				m.del(v.key, id, Expire)
			}
		}
		return false
	}

	expiredFound := 0

	for i := 0; i < totalChecks; i++ {
		sz := len(m.keys)
		id := rand.Intn(sz)
		v := m.values[id]
		if v.due <= m.now() {
			m.del(v.key, id, Expire)
			expiredFound++
		}
	}

	return expiredFound*4 >= totalChecks
}

// rotateExpire checks keys sequentially for expiration.
// Some keys may live too long, because randomExpire cannot hit them, and
// that's why this method was written. Basically, it iterates over 20 keys
// and checks them. The passed variable is k-th key, which previously was
// checked.
func (m *ExpireMap) rotateExpire(kth int) int {
	const totalChecks = 20
	if m.stopped {
		return 0
	}
	sz := len(m.keys)
	if sz == 0 {
		return 0
	}
	if kth >= sz || kth <= 0 {
		kth = sz - 1
	}
	for i := 0; i < totalChecks; i++ {
		v := m.values[kth]
		if v.due <= m.now() {
			m.del(v.key, kth, Expire)
		}
		kth--
		if kth < 0 {
			break
		}
	}
	return kth
}

// Curtime returns current time in Unix nanoseconds
func (m *ExpireMap) Curtime() int64 {
	return m.now()
}

// start starts two goroutines - first for updating curtime variable and
// second for expiration of keys. for loops with time.Sleep are used instead
// of time tickers.
func (m *ExpireMap) start() {
	go func() {
		kth := 0
		for !m.Stopped() {
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
		keys:   make(map[interface{}]int),
		values: nil,
		now:    func() int64 { return time.Now().UnixNano() },
	}
	rl.start()
	return rl
}
