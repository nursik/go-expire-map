package expiremap

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func BenchmarkExpireMap_Get(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Hour

	for _, pN := range presetN {
		pNN := pN
		b.Run(fmt.Sprintf("N_%d", pNN), func(b *testing.B) {
			b.StopTimer()
			expireMap := New()
			for j := 0; j < pNN; j++ {
				expireMap.Set(j, j, ttl)
			}
			b.StartTimer()
			for i := 0; i < b.N; i++ {
				_, _ = expireMap.Get(i % pNN)
			}
			b.StopTimer()
			expireMap.Close()
			expireMap = New()
			runtime.GC()
			b.StartTimer()
			expireMap.Close()
		})
	}
}

func BenchmarkExpireMap_Set(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Hour

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
				expireMap = New()
				runtime.GC()
				b.StartTimer()
			}
			expireMap.Close()
		})
	}
}

func BenchmarkExpireMap_Set2(b *testing.B) {
	presetN := []int{1000, 10000, 100000, 1000000, 10000000}

	ttl := time.Hour

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

	ttl := time.Hour

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

	ttl := time.Hour

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

	ttl := time.Hour

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
					expireMap.SetTTL(j, time.Nanosecond)
				}
				expireMap.Close()
			}
		})
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

func writer(expireMap *ExpireMap, wg *sync.WaitGroup, steps, N int, ttl time.Duration) {
	for steps >= 0 {
		steps--
		r := rand.Int()
		switch r % 4 {
		case 3:
			expireMap.Set(r%N, r%N, ttl)
		case 0:
			expireMap.Set(r%N, r%N, ttl)
		case 1:
			expireMap.Delete(r % N)
		case 2:
			expireMap.SetTTL(r%N, ttl)
		}
	}
	wg.Done()
}
