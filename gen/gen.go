package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
)

var key = flag.String("k", "interface{}", "name of the key struct")
var value = flag.String("v", "interface{}", "name of the value struct")
var zerovalue = flag.String("zv", "", "zero value (for pointers - type nil, struct T - type &T{})")

var outTemplate = `
// Auto-generated by github.com/nursik/go-expire-map/gen/gen.go
package expiremap

import (
	"github.com/nursik/go-ordered-set"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const timeResolution = time.Millisecond

const expireInterval = 100 * time.Millisecond

type KeyValue struct {
	Key   {{ .Key}}
	Value {{ .Value}}
}

type item struct {
	ttl   int64
	key   {{ .Key}}
	value {{ .Value}}
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
		page.values[index&(1<<pageBitSize-1)].value = {{ .ValueZero}}
	}
}

func (ps *pages) get(index uint64) item {
	bucket := index >> pageBitSize
	return ps.pages[bucket].values[index&(1<<pageBitSize-1)]
}

type ExpireMap struct {
	keys    map[{{ .Key}}]uint64
	values  *pages
	indices orderedset.OrderedSet
	mutex   sync.RWMutex
	stopped int64
	curtime int64
}

func (emp *ExpireMap) SetTTL(key {{ .Key}}, due time.Time) ({{ .Value}}, bool) {
	ttl := due.UnixNano()
	if ttl <= emp.Curtime() {
		emp.Delete(key)
		return {{ .ValueZero}}, false
	}
	emp.mutex.Lock()
	if emp.Stopped() {
		emp.mutex.Unlock()
		return {{ .ValueZero}}, false
	}
	id, ok := emp.keys[key]
	if ok == false {
		emp.mutex.Unlock()
		return {{ .ValueZero}}, false
	}
	v := emp.values.get(id)
	if v.ttl <= emp.Curtime() {
		emp.del(key, id)
		emp.mutex.Unlock()
		return {{ .ValueZero}}, false
	}
	v.ttl = ttl
	emp.values.put(id, v)
	emp.mutex.Unlock()
	return v.value, true
}

func (emp *ExpireMap) Get(key {{ .Key}}) ({{ .Value}}, bool) {
	emp.mutex.RLock()
	if emp.Stopped() {
		emp.mutex.RUnlock()
		return {{ .ValueZero}}, false
	}
	id, ok := emp.keys[key]
	if ok == false {
		emp.mutex.RUnlock()
		return {{ .ValueZero}}, false
	}
	v := emp.values.get(id)
	if v.ttl > emp.Curtime() {
		emp.mutex.RUnlock()
		return v.value, true
	}
	emp.mutex.RUnlock()
	emp.mutex.Lock()
	if emp.Stopped() {
		emp.mutex.Unlock()
		return {{ .ValueZero}}, false
	}

	id, ok = emp.keys[key]
	if ok == false {
		emp.mutex.Unlock()
		return {{ .ValueZero}}, false
	}

	v = emp.values.get(id)
	if v.ttl > emp.Curtime() {
		emp.mutex.Unlock()
		return v.value, true
	}

	emp.del(key, id)
	emp.mutex.Unlock()
	return {{ .ValueZero}}, false
}

func (emp *ExpireMap) GetTTL(key {{ .Key}}) int64 {
	emp.mutex.RLock()
	if emp.Stopped() {
		emp.mutex.RUnlock()
		return 0
	}
	id, ok := emp.keys[key]
	if ok == false {
		emp.mutex.RUnlock()
		return 0
	}
	v := emp.values.get(id)
	if v.ttl > emp.Curtime() {
		ttl := v.ttl
		emp.mutex.RUnlock()
		return ttl
	}
	emp.mutex.RUnlock()
	return 0
}

func (emp *ExpireMap) Delete(key {{ .Key}}) {
	emp.mutex.Lock()
	if emp.Stopped() {
		emp.mutex.Unlock()
		return
	}
	if id, ok := emp.keys[key]; ok {
		emp.del(key, id)
	}
	emp.mutex.Unlock()
}

func (emp *ExpireMap) Close() {
	emp.mutex.Lock()
	if emp.Stopped() == false {
		atomic.StoreInt64(&emp.stopped, 1)
		emp.keys = nil
		emp.values = nil
		emp.indices = nil
	}
	emp.mutex.Unlock()
}

func (emp *ExpireMap) Set(key {{ .Key}}, value {{ .Value}}, due time.Time) {
	ttl := due.UnixNano()
	if ttl <= emp.Curtime() {
		return
	}
	emp.mutex.Lock()
	if emp.Stopped() {
		emp.mutex.Unlock()
		return
	}

	id, ok := emp.keys[key]
	if !ok {
		id = emp.indices.LowestUnused()
		emp.indices.Insert(id)
		emp.keys[key] = id
	}
	emp.values.put(id, item{
		key:   key,
		value: value,
		ttl:   ttl,
	})
	emp.mutex.Unlock()
}

func (emp *ExpireMap) GetAll() []KeyValue {
	emp.mutex.RLock()
	if emp.Stopped() {
		emp.mutex.RUnlock()
		return nil
	}
	sz := emp.indices.Size()
	ans := make([]KeyValue, 0, sz)
	curtime := emp.Curtime()
	for i := 0; i < sz; i++ {
		id := emp.indices.Kth(i)
		v := emp.values.get(id)
		if v.ttl > curtime {
			ans = append(ans, KeyValue{Key: v.key, Value: v.value})
		}
	}
	emp.mutex.RUnlock()
	return ans
}

func (emp *ExpireMap) Size() int {
	emp.mutex.RLock()
	sz := len(emp.keys)
	emp.mutex.RUnlock()
	return sz
}

func (emp *ExpireMap) Stopped() bool {
	return atomic.LoadInt64(&emp.stopped) == 1
}

func (emp *ExpireMap) Curtime() int64 {
	return atomic.LoadInt64(&emp.curtime)
}

func (emp *ExpireMap) del(key {{ .Key}}, id uint64) {
	delete(emp.keys, key)
	emp.values.remove(id)
	emp.indices.Remove(id)
}

func (emp *ExpireMap) randomExpire() bool {
	const totalChecks = 20
	const bruteForceThreshold = 100
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
			expiredFound++
		}
	}

	if expiredFound*4 >= totalChecks {
		return true
	}
	return false
}

func (emp *ExpireMap) rotateExpire(kth int) int {
	const totalChecks = 20
	if emp.Stopped() {
		return 0
	}
	sz := emp.indices.Size()
	if sz == 0 {
		return 0
	}
	if kth >= sz || kth <= 0 {
		kth = sz - 1
	}
	for i := 0; i < totalChecks; i++ {
		id := emp.indices.Kth(kth)
		v := emp.values.get(id)
		if v.ttl <= emp.Curtime() {
			emp.del(v.key, id)
		}
		kth--
		if kth < 0 {
			break
		}
	}
	return kth
}

func (emp *ExpireMap) start() {
	go func() {
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
				emp.mutex.Lock()
				if !emp.randomExpire() {
					emp.mutex.Unlock()
					break
				}
				emp.mutex.Unlock()
			}
			emp.mutex.Lock()
			kth = emp.rotateExpire(kth)
			emp.mutex.Unlock()
			diff := time.Since(start)
			time.Sleep(expireInterval - diff)
		}
	}()
}

func New() *ExpireMap {
	rl := &ExpireMap{
		keys:    make(map[{{ .Key}}]uint64),
		indices: orderedset.NewTreeSet(),
		values:  &pages{},
		curtime: time.Now().UnixNano(),
	}
	rl.start()
	return rl
}

`

type R struct {
	Key string
	Value string
	ValueZero string
}

func main() {
	flag.Parse()

	r := R{
		Key: *key,
		Value: *value,
		ValueZero: "&" + *value + "{}",
	}
	if v := *value; v == "interface{}" || strings.HasPrefix(v, "*") {
		r.ValueZero = "nil"
	}
	if *zerovalue != "" {
		r.ValueZero = *zerovalue
	}
	fmt.Fprintf(os.Stderr, "Key - %s, Value - %s, Zero Value - %s\nWriting results to stdout\n", r.Key, r.Value, r.ValueZero)

	t, err := template.New("output").Parse(outTemplate)

	if err != nil {
		log.Fatal(err)
	}

	err = t.Execute(os.Stdout, r)

	if err != nil {
		log.Fatal(err)
	}
}
