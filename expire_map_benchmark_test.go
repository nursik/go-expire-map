package expiremap

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

type nativeMap struct {
	mp map[interface{}]interface{}
	sync.RWMutex
}

func (nativeMap) Expire(key interface{}, due time.Time) bool {
	panic("implement me")
}

func (n *nativeMap) Get(key interface{}) (interface{}, bool) {
	n.RLock()
	val, ok := n.mp[key]
	n.RUnlock()
	return val, ok
}

func (nativeMap) Delete(key interface{}) {
	panic("implement me")
}

func (n *nativeMap) Close() {
	n.Lock()
	n.mp = nil
	n.Unlock()
}

func (n *nativeMap) SetEx(key interface{}, value interface{}, due time.Time) {
	n.Lock()
	n.mp[key] = value
	n.Unlock()
}

func (nativeMap) GetAll() []KeyValue {
	panic("implement me")
}

func (nativeMap) Size() int {
	panic("implement me")
}

func (nativeMap) Curtime() int64 {
	panic("implement me")
}

func (nativeMap) Stopped() bool {
	panic("implement me")
}


func runBenchForEach(b *testing.B, f func(bb *testing.B, expireMap ExpireMap)) {
	var expireMap ExpireMap
	var ok bool

	// -- Redis Like Expire Map
	expireMap = NewRLExpireMap()
	ok = b.Run("RLExpireMap", func(b *testing.B) {
		f(b, expireMap)
	})
	if !ok {
		b.FailNow()
		expireMap.Close()
		return
	}
	expireMap.Close()
	expireMap = nil
	runtime.GC()
	// -- Redis Like Expire Map

	// -- Max TTL Expire Map
	expireMap = NewMTExpireMap(86400, 1)
	ok = b.Run("MTExpireMap", func(b *testing.B) {
		f(b, expireMap)
	})
	if !ok {
		b.FailNow()
	}
	expireMap.Close()
	expireMap = nil
	runtime.GC()
	// -- Max TTL Expire Map


	// -- Native map
	//expireMap = &nativeMap{mp: make(map[interface{}]interface{})}
	//ok = b.Run("NativeMap", func(b *testing.B) {
	//	f(b, expireMap)
	//})
	//if !ok {
	//	b.FailNow()
	//}
	//runtime.GC()
	// -- Native map
}

func runBenchForEachWithNew(b *testing.B, f func(bb *testing.B, NewMap func() ExpireMap)) {
	var ok bool

	// -- Redis Like Expire Map
	ok = b.Run("RLExpireMap", func(b *testing.B) {
		var expireMap ExpireMap
		f(b, func() ExpireMap {
			if expireMap != nil {
				expireMap.Close()
			}
			expireMap = NewRLExpireMap()
			return expireMap
		})
		if expireMap != nil {
			expireMap.Close()
		}
	})
	if !ok {
		b.FailNow()
		return
	}
	runtime.GC()
	// -- Redis Like Expire Map

	// -- Max TTL Expire Map
	ok = b.Run("MTExpireMap", func(b *testing.B) {
		var expireMap ExpireMap
		f(b, func() ExpireMap {
			if expireMap != nil {
				expireMap.Close()
			}
			expireMap = NewMTExpireMap(86400, 1)
			return expireMap
		})
		if expireMap != nil {
			expireMap.Close()
		}
	})
	if !ok {
		b.FailNow()
		return
	}
	runtime.GC()
	// -- Max TTL Expire Map
}

func BenchmarkSetEx(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			runBenchForEachWithNew(b, func(b *testing.B, NewMap func() ExpireMap) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					runtime.GC()
					expireMap := NewMap()
					b.StartTimer()
					for j := 0; j < pNN; j++ {
						expireMap.SetEx(i * pNN + j, i, ttl)
					}
				}
			})
		})
	}
}

func BenchmarkSetEx2(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			runBenchForEach(b, func(b *testing.B, expireMap ExpireMap) {
				for j := 0; j < pNN; j++ {
					expireMap.SetEx(j, j, ttl)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for j := 0; j < pNN; j++ {
						expireMap.SetEx(j, j, ttl)
					}
				}
			})
		})
	}
}

func BenchmarkDelete(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			runBenchForEachWithNew(b, func(b *testing.B, NewMap func() ExpireMap) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					runtime.GC()
					expireMap := NewMap()
					for j := 0; j < pNN; j++ {
						expireMap.SetEx(i * pNN + j, i, ttl)
					}
					b.StartTimer()
					for j := 0; j < pNN; j++ {
						expireMap.Delete(i * pNN + j)
					}
				}
			})
		})
	}
}

func BenchmarkExpire(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			runBenchForEach(b, func(b *testing.B, expireMap ExpireMap) {
				for j := 0; j < pNN; j++ {
					expireMap.SetEx(j, j, ttl)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					for j := 0; j < pNN; j++ {
						expireMap.Expire(j, ttl)
					}
				}
			})
		})
	}
}

func BenchmarkExpire2(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)
	oldttl := time.Now()

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			runBenchForEachWithNew(b, func(b *testing.B, NewMap func() ExpireMap) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					runtime.GC()
					expireMap := NewMap()
					for j := 0; j < pNN; j++ {
						expireMap.SetEx(i * pNN + j, i, ttl)
					}
					b.StartTimer()
					for j := 0; j < pNN; j++ {
						expireMap.Expire(i * pNN + j, oldttl)
					}
				}
			})
		})
	}
}

func BenchmarkGet(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}
	expiredRotation := []int{2, 3, 4, 5, 10, math.MaxInt32}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		for _, kP := range expiredRotation {
			pNN := pN
			kPP := kP
			ratio := strconv.Itoa(100 / kPP)
			b.Run(fmt.Sprintf("N_%d_Exp_%s%%", pNN, ratio), func(b *testing.B) {
				runBenchForEach(b, func(b *testing.B, expireMap ExpireMap) {
					for i := 0; i < b.N; i++ {
						b.StopTimer()
						for k := 0; k < pNN; k++ {
							if k% kPP == 0 {
								expireMap.SetEx(k, k, time.Now().Add(time.Millisecond))
							} else {
								expireMap.SetEx(k, k, ttl)
							}
						}
						time.Sleep(time.Millisecond)
						b.StartTimer()
						for k := 0; k < pNN; k++ {
							v, ok := expireMap.Get(k)
							if k%kPP != 0 && (v != k || !ok) {
								b.Error("did not got value or presence flag was false")
								b.FailNow()
							}
						}
					}

				})
			})
		}
	}
}

func BenchmarkRWParallel(b *testing.B) {
	const steps = 100000
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}
	rcnt := []int{1, 2, 4}
	wcnt := []int{1, 2}

	for _, pN := range presetN {
		for _, rN := range rcnt {
			for _, wN := range wcnt {
				pNN := pN
				rNN := rN
				wNN := wN
				b.Run(fmt.Sprintf("N_%d_Rn_%d_Wn_%d_Steps_%d", pNN, rNN, wNN, steps), func(b *testing.B) {
					runBenchForEach(b, func(b *testing.B, expireMap ExpireMap) {
						for i := 0; i < b.N; i++ {
							var wg sync.WaitGroup
							wg.Add(rNN + wNN)
							for j := 0; j < rNN; j++ {
								go reader(expireMap, &wg, steps, pNN)
							}
							for j := 0; j < wNN; j++ {
								go writer(expireMap, &wg, steps, pNN, time.Second)
							}
							wg.Wait()
						}
					})
				})
			}
		}
	}
}

func BenchmarkRWParallel2(b *testing.B) {
	const steps = 100000
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}
	rcnt := []int{1, 2, 4}
	wcnt := []int{1, 2}

	for _, pN := range presetN {
		for _, rN := range rcnt {
			for _, wN := range wcnt {
				pNN := pN
				rNN := rN
				wNN := wN
				b.Run(fmt.Sprintf("N_%d_Rn_%d_Wn_%d_Steps_%d", pNN, rNN, wNN, steps), func(b *testing.B) {
					runBenchForEach(b, func(b *testing.B, expireMap ExpireMap) {
						for i := 0; i < b.N; i++ {
							var wg sync.WaitGroup
							wg.Add(rNN + wNN)
							for j := 0; j < rNN; j++ {
								go reader(expireMap, &wg, steps, pNN)
							}
							for j := 0; j < wNN; j++ {
								go writer(expireMap, &wg, steps, pNN, time.Millisecond * 10)
							}
							wg.Wait()
						}
					})
				})
			}
		}
	}
}


func reader(expireMap ExpireMap, wg *sync.WaitGroup, steps, N int) {
	for steps >= 0 {
		steps--
		r := rand.Intn(N)
		v, ok := expireMap.Get(r)
		if ok && v != r {
			log.Panicf("got wrong value %v, %v, %v", ok, v, r)
		}
	}
	wg.Done()
}

func writer(expireMap ExpireMap, wg *sync.WaitGroup, steps, N int, ttlDiff time.Duration) {
	for steps >= 0 {
		steps--
		r := rand.Int()
		switch r%4 {
		case 3:
			expireMap.SetEx(r%N, r%N, time.Now().Add(ttlDiff))
		case 0:
			expireMap.SetEx(r%N, r%N, time.Now().Add(ttlDiff))
		case 1:
			expireMap.Delete(r%N)
		case 2:
			expireMap.Expire(r%N, time.Now().Add(ttlDiff))
		}
	}
	wg.Done()
}