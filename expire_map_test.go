package expiremap

import (
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"
)

const sleepTimeForExp = timeResolution + time.Millisecond

func TestExpireMap_Get(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	// Test 1 - Test basic get - check that it returns a proper value
	// and does not return an expired key
	expireMap.Set("key", "value", time.Second)

	if v, ok := expireMap.Get("key"); !ok || v != "value" {
		t.Error("Get() - no key or wrong value")
	}

	time.Sleep(time.Second + sleepTimeForExp)

	if v, ok := expireMap.Get("key"); ok || v != nil {
		t.Error("Get() - got key or wrong value")
	}
	// End of test 1

	// Test 2 - Test multiple gets
	ttl := time.Second

	for i := 0; i < 100000; i++ {
		expireMap.Set(i, i, ttl)
	}

	for i := 0; i < 100000; i++ {
		if v, ok := expireMap.Get(i); !ok || v != i {
			t.Error("Get() - no key or wrong value")
		}
	}

	time.Sleep(time.Second + sleepTimeForExp)

	for i := 0; i < 100000; i++ {
		if v, ok := expireMap.Get(i); ok || v != nil {
			t.Error("Get() - got key or wrong value")
		}
	}
	// End of test 2
}

func TestExpireMap_Get2(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()

	ttl := time.Hour
	ar := make([]int, 3, 3)
	ar[0] = 3

	expireMap.Set(1, ar, ttl)
	v, _ := expireMap.Get(1)

	v.([]int)[0] = 4

	if ar[0] != 4 {
		t.Error("Get() - error with getting pointer")
	}

	x := 10
	expireMap.Set(1, &x, ttl)
	v, _ = expireMap.Get(1)

	*v.(*int) = 11

	if x != 11 {
		t.Error("Get() - error with getting pointer")
	}
}

func TestExpireMap_Set(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	// Test 1 - Test that Set stores ttl properly
	ttl := time.Second

	expireMap.Set("key", "value", ttl)

	time.Sleep(time.Second + sleepTimeForExp)

	if v, ok := expireMap.Get("key"); ok || v != nil {
		t.Error("Set() - ttl is not set properly")
	}
	// End of test 1

	// Test 2 - Test multiple sets - check that multiple sets work
	ttl = 10 * time.Second

	for i := 0; i < 50000; i++ {
		expireMap.Set(i, i, ttl)
	}

	for i := 0; i < 50000; i++ {
		if v, ok := expireMap.Get(i); !ok || v != i {
			t.Error("Set() - no key or wrong value")
		}
	}

	for i := 0; i < 50000; i++ {
		expireMap.Set(i+50000, i+50000, ttl)
	}

	for i := 0; i < 100000; i++ {
		if v, ok := expireMap.Get(i); !ok || v != i {
			t.Error("Set() - no key or wrong value")
		}
	}
	// End of test 2

	// Test 3 - check that Set updates values
	for i := 0; i < 100000; i++ {
		expireMap.Set(i, i+10000, ttl)
	}

	for i := 0; i < 100000; i++ {
		if v, ok := expireMap.Get(i); !ok || v != i+10000 {
			t.Error("Set() - no key or wrong value")
		}
	}
	// End of test 3
}

func TestExpireMap_GetTTL(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()

	if v := expireMap.GetTTL(1); v != 0 {
		t.Error("GetTTL() - got non zero ttl for non existing key")
	}
	ttl := time.Second
	expireMap.Set(1, 1, ttl)

	if v := expireMap.GetTTL(1); math.Abs(float64(v-int64(ttl/time.Nanosecond))) >= 4*float64(timeResolution) {
		t.Error("GetTTL() - got wrong ttl")
	}

	time.Sleep(time.Second + sleepTimeForExp)

	if v := expireMap.GetTTL(1); v != 0 {
		t.Error("GetTTL() - got non zero ttl for expired key")
	}
}

func TestExpireMap_Size(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	// Test 1 - Check that initial size is zero
	if sz := expireMap.Size(); sz != 0 {
		t.Errorf("Size() - got %v, want %v", sz, 0)
	}
	// End of test 1

	// Test 2 - Check that Size() call's return value
	// is the same as an amount of unique inserted keys
	ttl := 10 * time.Second

	for i := 0; i < 100000; i++ {
		expireMap.Set(i, i, ttl)
	}

	if sz := expireMap.Size(); sz != 100000 {
		t.Errorf("Size() - got %v, want %v", sz, 100000)
	}

	for i := 0; i < 100000; i++ {
		expireMap.Set(i, i, ttl)
	}

	if sz := expireMap.Size(); sz != 100000 {
		t.Errorf("Size() - got %v, want %v", sz, 100000)
	}

	// Test 2 continued - Check that after some time size still returns
	// the same value, as no methods altering map are called
	time.Sleep(2 * time.Second)

	if sz := expireMap.Size(); sz != 100000 {
		t.Errorf("Size() - got %v, want %v", sz, 100000)
	}
	// End of test 2

	// Test 3 - Check that after deleting all keys in the map, Size() returns 0
	for i := 0; i < 100000; i++ {
		expireMap.Delete(i)
	}

	if sz := expireMap.Size(); sz != 0 {
		t.Errorf("Size() - got %v, want %v", sz, 0)
	}
	// End of test 3

	// Test 4 - Check that after calling SetTTL() for all keys in the map, Size() returns 0
	ttl = time.Second
	for i := 0; i < 100000; i++ {
		expireMap.Set(i, i, ttl)
	}

	if sz := expireMap.Size(); sz != 100000 {
		t.Errorf("Size() - got %v, want %v", sz, 100000)
	}

	ttl = -sleepTimeForExp

	for i := 0; i < 100000; i++ {
		expireMap.SetTTL(i, ttl)
	}

	if sz := expireMap.Size(); sz != 0 {
		t.Errorf("Size() - got %v, want %v", sz, 0)
	}
	// End of test 4
}

func TestExpireMap_SetTTL(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	ttl := 10 * time.Second

	v, ok := expireMap.SetTTL("key", ttl)

	if v != nil || ok {
		t.Error("SetTTL() - should return nil, false")
	}

	expireMap.Set("key", "value", ttl)

	if v, ok = expireMap.SetTTL("key", ttl); !ok || v != "value" {
		t.Error("SetTTL() - should return \"value\", true")
	}

	if v, ok := expireMap.Get("key"); !ok || v != "value" {
		t.Error("SetTTL() - should get a value by the key")
	}

	expireMap.SetTTL("key", timeResolution)

	time.Sleep(timeResolution + sleepTimeForExp)

	if v, ok := expireMap.Get("key"); ok || v != nil {
		t.Error("SetTTL() - should not get a value by the key")
	}
	expireMap.Set("key", "value", ttl)

	if v, ok := expireMap.Get("key"); !ok || v != "value" {
		t.Error("SetTTL() - should get a value by the key")
	}

	if v, ok = expireMap.SetTTL("key", -2*timeResolution); v != nil || ok {
		t.Error("SetTTL() - should return nil, false")
	}

	if v, ok := expireMap.Get("key"); ok || v != nil {
		t.Error("SetTTL() - should not get a value by the key")
	}

	ttl = 10 * time.Second

	for i := 0; i < 100000; i++ {
		expireMap.Set(i, i, ttl)
	}

	for i := 0; i < 100000; i++ {
		if v, ok = expireMap.SetTTL(i, ttl); !ok || v != i {
			t.Errorf("SetTTL() - should return %d, true", i)
		}
	}

	ttl = time.Second

	for i := 0; i < 100000; i++ {
		if i%2 == 0 {
			if v, ok = expireMap.SetTTL(i, ttl); !ok || v != i {
				t.Errorf("SetTTL() - should return %d, true", i)
			}
		} else {
			expireMap.Delete(i)
			if v, ok = expireMap.SetTTL(i, ttl); ok || v != nil {
				t.Error("SetTTL() - should return nil, false")
			}
		}
	}
	time.Sleep(time.Second + sleepTimeForExp)
	for i := 0; i < 100000; i += 2 {
		if v, ok = expireMap.SetTTL(i, ttl); v != nil || ok {
			t.Error("SetTTL() - should return nil, false")
		}
	}
}

func TestExpireMap_Delete(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	// Test 1 - Test basic delete
	ttl := 10 * time.Second
	expireMap.Set("key", "value", ttl)
	expireMap.Delete("key")

	if v, ok := expireMap.Get("key"); ok || v != nil {
		t.Error("Delete() - did not delete a key")
	}
	// End of test 1

	// Test 2 - Test multiple delete
	for i := 0; i < 100000; i++ {
		expireMap.Set(i, i, ttl)
	}
	for i := 0; i < 100000; i += 2 {
		expireMap.Delete(i)
		expireMap.Delete(i)
		expireMap.Delete(i)
	}
	for i := 0; i < 100000; i += 2 {
		if _, ok := expireMap.Get(i); ok {
			t.Error("Delete() - did not delete a key")
		}
	}
	// End of test 2
}

func TestExpireMap_Delete2(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	ttl := time.Hour
	expireMap.Set(1, 1, ttl)

	for i := 0; i < 100; i++ {
		v := make([]byte, 100000000, 100000000)
		expireMap.Set(2, v, ttl)
		expireMap.Delete(2)
	}
}

func TestExpireMap_StoppedAndClose(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	if stopped := expireMap.Stopped(); stopped {
		t.Error("did not call Close(), but found that it is stopped")
	}
	expireMap.Close()
	if stopped := expireMap.Stopped(); !stopped {
		t.Error("called Close(), but found that it is not stopped")
	}
	// Test that multiple Close() calls works
	expireMap.Close()
	expireMap.Close()
	expireMap.Close()

	curTime := expireMap.Curtime()
	time.Sleep(sleepTimeForExp)
	if curTime != expireMap.Curtime() {
		t.Error("curtime still updating")
	}
}

func TestExpireMap_Curtime(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	const testTickerInterval = 10 * time.Microsecond
	const testDuration = 5 * time.Second
	const maxAverageDiff = timeResolution

	var tickerCalls int64
	var totalDiff int64
	var maxDiff int64

	ticker := time.NewTicker(testTickerInterval)
	tickerStartTime := time.Now()
	for tick := range ticker.C {
		testTickerTime := tick.UnixNano()
		mapTime := expireMap.Curtime()
		diff := testTickerTime - mapTime
		totalDiff += diff
		if diff > maxDiff {
			maxDiff = diff
		}
		tickerCalls++
		if time.Now().Sub(tickerStartTime) >= testDuration {
			break
		}
	}
	ticker.Stop()

	if totalDiff > tickerCalls*int64(maxAverageDiff) {
		t.Errorf("Curtime() - average diff exceeded: max average diff %.3f, average diff %.3f (in nanosecs)", float64(maxAverageDiff), float64(totalDiff)/float64(tickerCalls))
		return
	}
	t.Logf("Curtime() - average diff %.3f, max diff %d (in nanosecs)", float64(totalDiff)/float64(tickerCalls), maxDiff)
}

func TestExpireMap_Curtime_WithHeavyLoad(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	const testTickerInterval = timeResolution
	const testDuration = 10 * time.Second
	const maxAverageDiff = timeResolution
	const N = 1000000
	i := 0

	startTime := time.Now()
	go func() {
		for {
			i++
			if expireMap.Stopped() {
				break
			}
			r := rand.Int()
			ttlDiff := time.Duration(rand.Intn(100)) * testDuration / 100

			switch r % 4 {
			case 0:
				_, _ = expireMap.Get(r % N)
			case 1:
				_, _ = expireMap.Get(r % N)
			case 2:
				expireMap.Set(r%N, r%N, startTime.Sub(time.Now())+ttlDiff)
			case 3:
				expireMap.Delete(r % N)
			default:
				continue
			}
		}
	}()

	var tickerCalls int64
	var totalDiff int64
	var maxDiff int64

	ticker := time.NewTicker(testTickerInterval)
	tickerStartTime := time.Now()
	for tick := range ticker.C {
		testTickerTime := tick.UnixNano()
		mapTime := expireMap.Curtime()
		diff := testTickerTime - mapTime
		totalDiff += diff
		if diff > maxDiff {
			maxDiff = diff
		}
		tickerCalls++
		if time.Now().Sub(tickerStartTime) >= testDuration {
			break
		}
	}
	ticker.Stop()
	expireMap.Close()

	if totalDiff > tickerCalls*int64(maxAverageDiff) {
		t.Errorf("Curtime() - average diff exceeded: max average diff %.3f, average diff %.3f (in nanosecs)", float64(maxAverageDiff), float64(totalDiff)/float64(tickerCalls))
		return
	}
	t.Logf("CurtimeWithHeavyLoad() - number of map method calls %v, average diff %.3f, max diff %d (in nanosecs)", i, float64(totalDiff)/float64(tickerCalls), maxDiff)
}

func TestExpireMap_GetAll(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	// Test 1 - Test basic
	ttl1 := time.Second
	ttl2 := 5 * time.Second

	for i := 0; i < 1000; i++ {
		if i%2 == 1 {
			expireMap.Set(i, i, ttl1)
		} else {
			expireMap.Set(i, i, ttl2)
		}
	}

	kvs := expireMap.GetAll()

	if len(kvs) != 1000 {
		t.Errorf("GetAll() - slice length mismatch, got %v, want %v", len(kvs), 1000)
	}

	sort.Slice(kvs, func(i, j int) bool {
		if kvs[i].Key.(int) < kvs[j].Key.(int) {
			return true
		}
		return false
	})

	for i, kv := range kvs {
		if kv.Key != i || kv.Value != i {
			t.Error("GetAll() - slice does not contain expected values")
		}
	}
	// End of test 1

	// Test 2 - Test that it returns only non expired keys
	time.Sleep(time.Second + sleepTimeForExp)

	kvs = expireMap.GetAll()

	if len(kvs) != 500 {
		t.Errorf("GetAll() - slice length mismatch, got %v, want %v", len(kvs), 500)
		return
	}

	sort.Slice(kvs, func(i, j int) bool {
		if kvs[i].Key.(int) < kvs[j].Key.(int) {
			return true
		}
		return false
	})

	for i, kv := range kvs {
		if kv.Key != i*2 || kv.Value != i*2 {
			t.Error("GetAll() - slice does not contain expected values")
			return
		}
	}
	// End of test 2
}

func TestExpireMap_Notify(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()

	c := make(chan Event, 10)

	expireMap.Notify(c, AllEvents)
	expireMap.Set(1, 1, time.Second)

	if e := <-c; e.Key != 1 || e.Value != 1 || e.Type != Set {
		t.Error("Notify() - got wrong value")
	}

	expireMap.Set(1, 2, time.Second)

	if e := <-c; e.Key != 1 || e.Value != 2 || e.Type != Update {
		t.Error("Notify() - got wrong value")
	}

	expireMap.Set(2, 2, time.Second)

	if e := <-c; e.Key != 2 || e.Value != 2 || e.Type != Set {
		t.Error("Notify() - got wrong value")
	}

	expireMap.Delete(2)

	if e := <-c; e.Key != 2 || e.Value != 2 || e.Type != Delete {
		t.Error("Notify() - got wrong value")
	}

	time.Sleep(time.Second)

	if e := <-c; e.Key != 1 || e.Value != 2 || e.Type != Expire {
		t.Error("Notify() - got wrong value")
	}
}
