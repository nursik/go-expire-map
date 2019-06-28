package expiremap

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const billion = 1000000000
const mtExpireInterval = 100 * time.Millisecond
const maxMtKeyDeletes = 200

type mtQueueItem struct {
	key interface{}
	next *mtQueueItem
	prev *mtQueueItem
	queue *mtQueue
}

type mtQueue struct {
	ttl int64
	head *mtQueueItem
}

func newMtQueue(ttl int64) *mtQueue {
	return &mtQueue{ttl: ttl}
}

func (q *mtQueue) new(key interface{}) *mtQueueItem {
	item := &mtQueueItem{
		key: key,
		next: q.head,
		queue: q,
	}

	if q.head != nil {
		q.head.prev = item
	}
	q.head = item

	return item
}

func (item *mtQueueItem) Remove() {
	// if item is a head of queue
	if item.prev == nil {
		// make queue head the next item
		item.queue.head = item.next
		// if next item is not nil, point it's prev to nil
		if item.next != nil {
			item.next.prev = nil
		}
	} else {
		// point prev item's next to current item's next
		item.prev.next = item.next
		if item.next != nil {
			item.next.prev = item.prev
		}
	}
}

type mtVal struct {
	ttl int64
	qitem *mtQueueItem
	value interface{}
}

type mtExpireMap struct {
	secToWait int64
	queueHead int
	ttlQueue []*mtQueue
	expiredQueue []*mtQueue
	cache map[interface{}]mtVal
	sync.RWMutex
	stopped int64
	curtime int64
}

func (mt *mtExpireMap) Expire(key interface{}, due time.Time) bool {
	ttl := due.UnixNano()
	if ttl <= mt.Curtime() {
		mt.Delete(key)
		return false
	}
	mt.Lock()
	if mt.Stopped() {
		mt.Unlock()
		return false
	}

	v, ok := mt.cache[key]
	if !ok {
		mt.Unlock()
		return false
	}
	v.qitem.Remove()
	delete(mt.cache, key)

	if v.ttl <= mt.Curtime() {
		mt.Unlock()
		return false
	}
	qlen := len(mt.ttlQueue)
	qoffset := int(ttl / billion - mt.ttlQueue[mt.queueHead].ttl)

	if qoffset >= qlen || qoffset < 0 {
		mt.Unlock()
		log.Panicln("trying to access queue array with invalid index")
		return false
	}

	mt.cache[key] = mtVal{
		ttl: ttl,
		value: v.value,
		qitem: mt.ttlQueue[qoffset % qlen].new(key),
	}

	mt.Unlock()
	return true
}

func (mt *mtExpireMap) Get(key interface{}) (interface{}, bool) {
	mt.RLock()
	if mt.Stopped() {
		mt.RUnlock()
		return nil, false
	}
	v, ok := mt.cache[key]
	if !ok || v.ttl <= mt.Curtime(){
		mt.RUnlock()
		return nil, false
	}
	mt.RUnlock()
	return v.value, true
}

func (mt *mtExpireMap) Delete(key interface{}) {
	mt.Lock()
	if mt.Stopped() {
		mt.Unlock()
		return
	}
	v, ok := mt.cache[key]
	if !ok {
		mt.Unlock()
		return
	}
	v.qitem.Remove()
	delete(mt.cache, key)
	mt.Unlock()
}

func (mt *mtExpireMap) Close() {
	mt.Lock()
	if !mt.Stopped() {
		atomic.StoreInt64(&mt.stopped, 1)
		mt.ttlQueue = nil
		mt.expiredQueue = nil
		mt.cache = nil
	}
	mt.Unlock()
}

func (mt *mtExpireMap) SetEx(key interface{}, value interface{}, due time.Time) {
	ttl := due.UnixNano()

	if ttl <= mt.Curtime() {
		return
	}
	mt.Lock()
	if mt.Stopped() {
		mt.Unlock()
		return
	}
	qlen := len(mt.ttlQueue)
	qoffset := int(ttl / billion - mt.ttlQueue[mt.queueHead].ttl)

	if qoffset >= qlen || qoffset < 0 {
		mt.Unlock()
		log.Panicln("trying to access queue array with invalid index")
		return
	}

	if v, ok := mt.cache[key]; ok {
		v.qitem.Remove()
	}
	mt.cache[key] = mtVal{
		ttl: ttl,
		value: value,
		qitem: mt.ttlQueue[qoffset % qlen].new(key),
	}
	mt.Unlock()
}

func (mt *mtExpireMap) GetAll() []KeyValue {
	mt.RLock()
	if mt.Stopped() {
		mt.RUnlock()
		return nil
	}
	ans := make([]KeyValue, 0, len(mt.cache))
	curtime := mt.Curtime()
	for k, val := range mt.cache {
		if curtime >= val.ttl {
			continue
		}
		ans = append(ans, KeyValue{
			key: k,
			value: val.value,
		})
	}
	mt.RUnlock()
	return ans
}

func (mt *mtExpireMap) Size() int {
	mt.RLock()
	if mt.Stopped() {
		mt.RUnlock()
		return 0
	}
	sz := len(mt.cache)
	mt.RUnlock()
	return sz
}

func (mt *mtExpireMap) Curtime() int64 {
	return atomic.LoadInt64(&mt.curtime)
}

func (mt *mtExpireMap) expire() {
	if mt.Stopped() {
		return
	}
	qlen := len(mt.ttlQueue)
	for {
		curQueue := mt.ttlQueue[mt.queueHead]
		prevQueueTtl := mt.ttlQueue[(mt.queueHead + qlen - 1)%qlen].ttl
		if (curQueue.ttl + mt.secToWait) * billion <= mt.Curtime() {
			mt.expiredQueue = append(mt.expiredQueue, curQueue)
			mt.ttlQueue[mt.queueHead] = newMtQueue(prevQueueTtl + mt.secToWait)
			mt.queueHead = (mt.queueHead + 1) %qlen
		} else {
			break
		}
	}
	d := maxMtKeyDeletes
	for i, q := range mt.expiredQueue {
		if q == nil {
			continue
		}
		cur := q.head

		for cur != nil {
			d--
			delete(mt.cache, cur.key)
			cur = cur.next
			if d == 0 {
				break
			}
		}
		if cur == nil {
			mt.expiredQueue[i] = nil
		}
	}
	nonEmptyQueuesCnt := 0

	for _, q := range mt.expiredQueue {
		if q != nil {
			nonEmptyQueuesCnt++
		}
	}
	newExpiredQueue := make([]*mtQueue, 0, 2*nonEmptyQueuesCnt)
	for _, q := range mt.expiredQueue {
		if q != nil {
			newExpiredQueue = append(newExpiredQueue, q)
		}
	}
	mt.expiredQueue = newExpiredQueue
}

func (mt *mtExpireMap) Stopped() bool {
	return atomic.LoadInt64(&mt.stopped) == 1
}

func (mt *mtExpireMap) start() {
	go func(){
		for {
			if mt.Stopped() {
				break
			}
			atomic.StoreInt64(&mt.curtime, time.Now().UnixNano())
			time.Sleep(timeResolution)
		}
	}()

	go func() {
		for {
			if mt.Stopped() {
				break
			}
			start := time.Now()
			mt.Lock()
			mt.expire()
			mt.Unlock()
			diff := time.Since(start)
			time.Sleep(mtExpireInterval - diff)
		}
	}()
}

func NewMTExpireMap(maxTtl, secToWait int) ExpireMap {
	if secToWait <= 0 {
		panic("secToWait must be positive integer")
	}
	if maxTtl <= 0 {
		panic("maxTtl must be positive integer")
	}
	ttlQueueLen := maxTtl / secToWait + 5
	if maxTtl %secToWait != 0 {
		ttlQueueLen++
	}
	curtime := time.Now().UnixNano()

	mt := &mtExpireMap{
		secToWait: int64(secToWait),
		queueHead: 0,
		cache: make(map[interface{}]mtVal),
		ttlQueue: make([]*mtQueue, ttlQueueLen, ttlQueueLen),
		curtime: curtime,
	}

	curTimeInSec := curtime / billion

	for i := 0; i < ttlQueueLen; i++ {
		mt.ttlQueue[i] = newMtQueue(curTimeInSec + int64(i * secToWait))
	}

	mt.start()

	return mt
}