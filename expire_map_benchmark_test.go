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

func BenchmarkExpireMap_Set(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			expireMap := New()
			for i := 0; i < b.N; i++ {
				for j := 0; j < pNN; j++ {
					expireMap.Set(j, j, ttl)
				}
				b.StopTimer()
				expireMap.Close()
				runtime.GC()
				expireMap = New()
				b.StartTimer()
			}
			expireMap.Close()
		})
	}
}

func BenchmarkExpireMap_Set2(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			expireMap := New()
			for j := 0; j < pNN; j++ {
				expireMap.Set(j, j, ttl)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := 0; j < pNN; j++ {
					expireMap.Set(j, j, ttl)
				}
			}
			expireMap.Close()
		})
	}
}

func BenchmarkExpireMap_Delete(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				runtime.GC()
				expireMap := New()
				for j := 0; j < pNN; j++ {
					expireMap.Set(j, j, ttl)
				}
				b.StartTimer()
				for j := 0; j < pNN; j++ {
					expireMap.Delete(j)
				}
				expireMap.Close()
			}
		})
	}
}

func BenchmarkExpireMap_SetTTL(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			expireMap := New()
			for j := 0; j < pNN; j++ {
				expireMap.Set(j, j, ttl)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := 0; j < pNN; j++ {
					expireMap.SetTTL(j, ttl)
				}
			}
			expireMap.Close()
		})
	}
}

func BenchmarkExpireMap_SetTTL2(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Now().Add(time.Hour)
	oldttl := time.Now()

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				runtime.GC()
				expireMap := New()
				for j := 0; j < pNN; j++ {
					expireMap.Set(j, j, ttl)
				}
				b.StartTimer()
				for j := 0; j < pNN; j++ {
					expireMap.SetTTL(j, oldttl)
				}
				expireMap.Close()
			}
		})
	}
}

func BenchmarkExpireMap_Get(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}
	expiredRotation := []int{2, 3, 4, 5, 10, math.MaxInt32}

	ttl := time.Now().Add(time.Hour)

	for _, pN := range presetN {
		for _, eP := range expiredRotation {
			pNN := pN
			ePP := eP
			ratio := strconv.Itoa(100 / ePP)
			b.Run(fmt.Sprintf("N_%d_Exp_%s%%", pNN, ratio), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					runtime.GC()
					expireMap := New()
					for j := 0; j < pNN; j++ {
						if j%ePP == 0 {
							expireMap.Set(j, j, time.Now().Add(time.Millisecond))
						} else {
							expireMap.Set(j, j, ttl)
						}
					}
					time.Sleep(time.Millisecond)
					b.StartTimer()
					for j := 0; j < pNN; j++ {
						v, ok := expireMap.Get(j)
						if j%ePP != 0 && (v != j || !ok) {
							b.Error("did not got value or presence flag was false")
							b.FailNow()
						}
					}
					expireMap.Close()
				}
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
					expireMap := New()
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
					expireMap.Close()
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
					expireMap := New()
					for i := 0; i < b.N; i++ {
						var wg sync.WaitGroup
						wg.Add(rNN + wNN)
						for j := 0; j < rNN; j++ {
							go reader(expireMap, &wg, steps, pNN)
						}
						for j := 0; j < wNN; j++ {
							go writer(expireMap, &wg, steps, pNN, time.Millisecond*10)
						}
						wg.Wait()
					}
					expireMap.Close()
				})
			}
		}
	}
}

func reader(expireMap *ExpireMap, wg *sync.WaitGroup, steps, N int) {
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

func writer(expireMap *ExpireMap, wg *sync.WaitGroup, steps, N int, ttlDiff time.Duration) {
	for steps >= 0 {
		steps--
		r := rand.Int()
		switch r % 4 {
		case 3:
			expireMap.Set(r%N, r%N, time.Now().Add(ttlDiff))
		case 0:
			expireMap.Set(r%N, r%N, time.Now().Add(ttlDiff))
		case 1:
			expireMap.Delete(r % N)
		case 2:
			expireMap.SetTTL(r%N, time.Now().Add(ttlDiff))
		}
	}
	wg.Done()
}
