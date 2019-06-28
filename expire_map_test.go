package expiremap

import (
	"math/rand"
	"runtime"
	"sort"
	"testing"
	"time"
)

const TestTimeResolutionErrorDiff = timeResolution + time.Millisecond


func runTestFunctionForEachMap(t *testing.T, f func(tt *testing.T, expireMap ExpireMap)) {
	var expireMap ExpireMap
	var ok bool

	// -- Redis Like Expire Map
	expireMap = NewRLExpireMap()
	ok = t.Run("RLExpireMap", func(t *testing.T) {
		f(t, expireMap)
	})
	if !ok {
		expireMap.Close()
		t.FailNow()
	}
	expireMap.Close()
	runtime.GC()
	// -- Redis Like Expire Map

	// -- Max TTL Expire Map
	expireMap = NewMTExpireMap(86400, 5)
	ok = t.Run("MTExpireMap", func(t *testing.T) {
		f(t, expireMap)
	})
	if !ok {
		expireMap.Close()
		t.FailNow()
	}
	expireMap.Close()
	runtime.GC()
	// -- Max TTL Expire Map
}

func TestGet(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
		// Test 1 - Test basic get - check that it returns a proper value
		// and does not return an expired key
		expireMap.SetEx("key", "value", time.Now().Add(timeResolution))

		if v, ok := expireMap.Get("key"); !ok || v != "value" {
			t.Error("Get() - no key or wrong value")
		}

		time.Sleep(timeResolution + TestTimeResolutionErrorDiff)

		if v, ok := expireMap.Get("key"); ok || v != nil {
			t.Error("Get() - got key or wrong value")
		}
		// End of test 1


		// Test 2 - Test multiple gets
		ttl := time.Now().Add(time.Second)

		for i := 0; i < 100000; i++ {
			expireMap.SetEx(i, i, ttl)
		}

		for i := 0; i < 100000; i++ {
			if v, ok := expireMap.Get(i); !ok || v != i {
				t.Error("Get() - no key or wrong value")
			}
		}

		time.Sleep(time.Second + TestTimeResolutionErrorDiff)

		for i := 0; i < 100000; i++ {
			if v, ok := expireMap.Get(i); ok || v != nil {
				t.Error("Get() - got key or wrong value")
			}
		}
		// End of test 2
	})
}

func TestSetEx(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
		// Test 1 - Test that SetEx stores ttl properly
		ttl := time.Now().Add(time.Second)

		expireMap.SetEx("key", "value", ttl)

		time.Sleep(time.Second + TestTimeResolutionErrorDiff)

		if v, ok := expireMap.Get("key"); ok || v != nil {
			t.Error("SetEx() - ttl is not set properly")
		}
		// End of test 1


		// Test 2 - Test multiple sets - check that multiple sets work
		ttl = time.Now().Add(10 * time.Second)

		for i := 0; i < 50000; i++ {
			expireMap.SetEx(i, i, ttl)
		}

		for i := 0; i < 50000; i++ {
			if v, ok := expireMap.Get(i); !ok || v != i {
				t.Error("SetEx() - no key or wrong value")
			}
		}

		for i := 0; i < 50000; i++ {
			expireMap.SetEx(i + 50000, i + 50000, ttl)
		}

		for i := 0; i < 100000; i++ {
			if v, ok := expireMap.Get(i); !ok || v != i {
				t.Error("SetEx() - no key or wrong value")
			}
		}
		// End of test 2


		// Test 3 - check that SetEx updates values
		for i := 0; i < 100000; i++ {
			expireMap.SetEx(i, i + 10000, ttl)
		}

		for i := 0; i < 100000; i++ {
			if v, ok := expireMap.Get(i); !ok || v != i + 10000 {
				t.Error("SetEx() - no key or wrong value")
			}
		}
		// End of test 3
	})
}

func TestSize(t  *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
		// Test 1 - Check that initial size is zero
		if sz := expireMap.Size(); sz != 0 {
			t.Errorf("Size() - got %v, want %v", sz, 0)
		}
		// End of test 1


		// Test 2 - Check that Size() call's return value
		// is the same as an amount of unique inserted keys
		ttl := time.Now().Add(10 * time.Second)

		for i := 0; i < 100000; i++ {
			expireMap.SetEx(i, i, ttl)
		}

		if sz := expireMap.Size(); sz != 100000 {
			t.Errorf("Size() - got %v, want %v", sz, 100000)
		}

		for i := 0; i < 100000; i++ {
			expireMap.SetEx(i, i, ttl)
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

		// Test 4 - Check that after calling Expire() for all keys in the map, Size() returns 0
		ttl = time.Now().Add(time.Second)
		for i := 0; i < 100000; i++ {
			expireMap.SetEx(i, i, ttl)
		}

		if sz := expireMap.Size(); sz != 100000 {
			t.Errorf("Size() - got %v, want %v", sz, 100000)
		}

		ttl = time.Now().Add( -TestTimeResolutionErrorDiff)

		for i := 0; i < 100000; i++ {
			expireMap.Expire(i, ttl)
		}

		if sz := expireMap.Size(); sz != 0 {
			t.Errorf("Size() - got %v, want %v", sz, 0)
		}
		// End of test 4
	})
}

func TestExpire(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
		ttl := time.Now().Add(10 * time.Second)

		ok := expireMap.Expire("key", ttl)

		if ok {
			t.Error("Expire() - should return false")
		}

		expireMap.SetEx("key", "value", ttl)

		if ok = expireMap.Expire("key", ttl); !ok {
			t.Error("Expire() - should return true")
		}

		if v, ok := expireMap.Get("key"); !ok || v != "value" {
			t.Error("Expire() - should get a value by the key")
		}

		expireMap.Expire("key", time.Now().Add(timeResolution))

		time.Sleep(timeResolution + TestTimeResolutionErrorDiff)

		if v, ok := expireMap.Get("key"); ok || v != nil{
			t.Error("Expire() - should not get a value by the key")
		}
		expireMap.SetEx("key", "value", ttl)

		if v, ok := expireMap.Get("key"); !ok || v != "value" {
			t.Error("Expire() - should get a value by the key")
		}

		if ok = expireMap.Expire("key", time.Now().Add(- 2 *timeResolution)); ok {
			t.Error("Expire() - should return false")
		}

		if v, ok := expireMap.Get("key"); ok || v != nil{
			t.Error("Expire() - should not get a value by the key")
		}

		ttl = time.Now().Add(10 * time.Second)

		for i := 0; i < 100000; i++ {
			expireMap.SetEx(i, i, ttl)
		}

		for i := 0; i < 100000; i++ {
			if ok = expireMap.Expire(i, ttl); !ok {
				t.Error("Expire() - should return true")
			}
		}

		ttl = time.Now().Add(time.Second)

		for i := 0; i < 100000; i++ {
			if i % 2 == 0 {
				if ok = expireMap.Expire(i, ttl); !ok {
					t.Error("Expire() - should return true")
				}
			} else {
				expireMap.Delete(i)
				if ok = expireMap.Expire(i, ttl); ok {
					t.Error("Expire() - should return false")
				}
			}
		}
		time.Sleep(time.Second + TestTimeResolutionErrorDiff)
		for i := 0; i < 100000; i += 2 {
			if ok = expireMap.Expire(i, ttl); ok {
				t.Error("Expire() - should return false")
			}
		}
	})
}

func TestDelete(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
		// Test 1 - Test basic delete
		ttl := time.Now().Add(10 * time.Second)
		expireMap.SetEx("key", "value", ttl)
		expireMap.Delete("key")

		if v, ok := expireMap.Get("key"); ok || v != nil {
			t.Error("Delete() - did not delete a key")
		}
		// End of test 1

		// Test 2 - Test multiple delete
		for i := 0; i < 100000; i++ {
			expireMap.SetEx(i, i, ttl)
		}
		for i := 0; i < 100000; i+=2 {
			expireMap.Delete(i)
			expireMap.Delete(i)
			expireMap.Delete(i)
		}
		for i := 0; i < 100000; i+=2 {
			if _, ok := expireMap.Get(i); ok {
				t.Error("Delete() - did not delete a key")
			}
		}
		// End of test 2
	})
}

func TestStoppedAndClose(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
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
		time.Sleep(TestTimeResolutionErrorDiff)
		if curTime != expireMap.Curtime() {
			t.Error("curtime still updating")
		}
	})
}

func TestCurtime(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
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

		if totalDiff > tickerCalls * int64(maxAverageDiff) {
			t.Errorf("Curtime() - average diff exceeded: max average diff %.3f, average diff %.3f (in nanosecs)", float64(maxAverageDiff), float64(totalDiff) / float64(tickerCalls))
			return
		}
		t.Logf("Curtime() - average diff %.3f, max diff %d (in nanosecs)", float64(totalDiff) / float64(tickerCalls), maxDiff)
	})
}


func TestCurtimeWithHeavyLoad(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
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

				switch r%4 {
				case 0:
					_, _ = expireMap.Get(r%N)
				case 1:
					_, _ = expireMap.Get(r%N)
				case 2:
					expireMap.SetEx(r%N, r%N, startTime.Add(ttlDiff))
				case 3:
					expireMap.Delete(r%N)
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

		if totalDiff > tickerCalls * int64(maxAverageDiff) {
			t.Errorf("Curtime() - average diff exceeded: max average diff %.3f, average diff %.3f (in nanosecs)", float64(maxAverageDiff), float64(totalDiff) / float64(tickerCalls))
			return
		}
		t.Logf("CurtimeWithHeavyLoad() - number of map method calls %v, average diff %.3f, max diff %d (in nanosecs)", i, float64(totalDiff) / float64(tickerCalls), maxDiff)
	})
}

func TestGetAll(t *testing.T) {
	runTestFunctionForEachMap(t, func(t *testing.T, expireMap ExpireMap) {
		// Test 1 - Test basic
		ttl1 := time.Now().Add(time.Second)
		ttl2 := time.Now().Add(2 * time.Second)

		for i := 0; i < 1000; i++ {
			if i%2 == 1 {
				expireMap.SetEx(i, i, ttl1)
			} else {
				expireMap.SetEx(i, i, ttl2)
			}
		}

		kvs := expireMap.GetAll()

		if len(kvs) != 1000 {
			t.Errorf("GetAll() - slice length mismatch, got %v, want %v", len(kvs), 1000)
		}

		sort.Slice(kvs, func(i, j int) bool {
			if kvs[i].key.(int) < kvs[j].key.(int) {
				return true
			}
			return false
		})

		for i, kv := range kvs {
			if kv.key != i || kv.value != i {
				t.Error("GetAll() - slice does not contain expected values")
			}
		}
		// End of test 1


		// Test 2 - Test that it returns only non expired keys
		time.Sleep(time.Second)

		kvs = expireMap.GetAll()

		if len(kvs) != 500 {
			t.Errorf("GetAll() - slice length mismatch, got %v, want %v", len(kvs), 500)
			return
		}

		sort.Slice(kvs, func(i, j int) bool {
			if kvs[i].key.(int) < kvs[j].key.(int) {
				return true
			}
			return false
		})

		for i, kv := range kvs {
			if kv.key != i*2 || kv.value != i*2 {
				t.Error("GetAll() - slice does not contain expected values")
				return
			}
		}
		// End of test 2
	})
}

