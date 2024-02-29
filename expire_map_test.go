package expiremap

import (
	"slices"
	"testing"
	"time"
)

func TestExpireMap_SetAndGet(t *testing.T) {
	em := New()
	defer em.Close()

	for i := 0; i < 1<<12; i++ {
		v, ok := em.Get(i)
		if ok || v != nil {
			t.Fatal("got value or ok")
		}
		em.Set(i, i, time.Hour)

		v, ok = em.Get(i)
		if !ok || v != i {
			t.Fatalf("got %v, %v - expected %v, %v", v, ok, i, true)
		}
	}
}

func TestExpireMap_SetAndGetExpired(t *testing.T) {
	em := New()
	defer em.Close()

	em.now = func() int64 { return 0 }
	for i := 0; i < 1<<12; i++ {
		em.Set(i, i, time.Duration(2-i%2))
	}

	for i := 0; i < 1<<12; i++ {
		v, ok := em.Get(i)
		if !ok || v != i {
			t.Fatalf("got %v, %v - expected %v, %v", v, ok, i, true)
		}
	}

	em.now = func() int64 { return 1 }

	for i := 0; i < 1<<12; i++ {
		v, ok := em.Get(i)
		if i%2 == 0 && (!ok || v != i) {
			t.Fatalf("got %v, %v - expected %v, %v", v, ok, i, true)
		}

		if i%2 == 1 && (ok || v != nil) {
			t.Fatalf("got %v, %v - expected %v, %v", v, ok, nil, false)
		}
	}

	em.now = func() int64 { return 2 }

	for i := 0; i < 1<<12; i++ {
		v, ok := em.Get(i)
		if ok || v != nil {
			t.Fatalf("got %v, %v - expected %v, %v", v, ok, nil, false)
		}
	}
}

func TestExpireMap_GetCheckDifferentTypes(t *testing.T) {
	em := New()
	defer em.Close()

	ttl := time.Hour
	ar := make([]int, 3)
	ar[0] = 3

	em.Set(1, ar, ttl)
	v, _ := em.Get(1)

	v.([]int)[0] = 4

	if ar[0] != 4 {
		t.Fatal("got invalid slice")
	}

	x := 10
	em.Set(1, &x, ttl)
	v, _ = em.Get(1)

	*v.(*int) = 11

	if x != 11 {
		t.Fatal("got invalid pointer")
	}
}

func TestExpireMap_GetTTL(t *testing.T) {
	em := New()
	defer em.Close()

	if v := em.GetTTL(1); v != 0 {
		t.Fatal("got non zero ttl for non existing key")
	}

	em.now = func() int64 { return 0 }

	ttl := time.Duration(1)
	em.Set(1, 1, ttl)

	em.now = func() int64 { return 2 }

	if v := em.GetTTL(1); v != 0 {
		t.Fatal("got non zero ttl for expired key")
	}
}

func TestExpireMap_Size(t *testing.T) {
	em := New()
	defer em.Close()
	if sz := em.Size(); sz != 0 {
		t.Fatalf("got %v, want %v", sz, 0)
	}

	em.now = func() int64 { return 0 }
	ttl := time.Duration(10)

	for i := 0; i < 100000; i++ {
		em.Set(i, i, ttl)
	}

	if sz := em.Size(); sz != 100000 {
		t.Fatalf("got %v, want %v", sz, 100000)
	}

	for i := 0; i < 100000; i++ {
		em.Set(i, i, ttl)
	}

	if sz := em.Size(); sz != 100000 {
		t.Fatalf("got %v, want %v", sz, 100000)
	}

	// Check that after some time size still returns
	// the same value, as no methods altering map are called
	em.now = func() int64 { return 2 }

	if sz := em.Size(); sz != 100000 {
		t.Fatalf("got %v, want %v", sz, 100000)
	}

	// Check that after deleting all keys in the map, Size() returns 0
	for i := 0; i < 100000; i++ {
		em.Delete(i)
	}

	if sz := em.Size(); sz != 0 {
		t.Fatalf("got %v, want %v", sz, 0)
	}

	// Test 4 - Check that after calling SetTTL() for all keys in the map, Size() returns 0
	ttl = time.Second
	for i := 0; i < 100000; i++ {
		em.Set(i, i, ttl)
	}

	if sz := em.Size(); sz != 100000 {
		t.Fatalf("got %v, want %v", sz, 100000)
	}

	ttl = 0

	for i := 0; i < 100000; i++ {
		em.SetTTL(i, ttl)
	}

	if sz := em.Size(); sz != 0 {
		t.Fatalf("got %v, want %v", sz, 0)
	}
}

func TestExpireMap_SetTTL(t *testing.T) {
	em := New()
	defer em.Close()
	ttl := time.Second

	v, ok := em.SetTTL("key", ttl)

	if v != nil || ok {
		t.Fatalf("got %v, %v - expected %v, %v", v, ok, nil, false)
	}

	em.Set("key", "value", ttl)

	if v, ok = em.SetTTL("key", ttl); !ok || v != "value" {
		t.Fatalf("got %v, %v - expected %v, %v", v, ok, "value", true)
	}

	if v, ok = em.Get("key"); !ok || v != "value" {
		t.Fatalf("got %v, %v - expected %v, %v", v, ok, "value", true)
	}

	em.SetTTL("key", 0)

	if v, ok := em.Get("key"); ok || v != nil {
		t.Fatalf("got %v, %v - expected %v, %v", v, ok, nil, false)
	}
}

func TestExpireMap_Delete(t *testing.T) {
	em := New()
	defer em.Close()
	ttl := 10 * time.Second

	em.Set("key", "value", ttl)
	em.Delete("key")

	if v, ok := em.Get("key"); ok || v != nil {
		t.Fatalf("got %v, %v - expected %v, %v", v, ok, nil, false)
	}

	for i := 0; i < 100000; i++ {
		em.Set(i, i, ttl)
	}
	for i := 0; i < 100000; i += 2 {
		em.Delete(i)
		em.Delete(i)
		em.Delete(i)
	}
	for i := 0; i < 100000; i += 2 {
		if v, ok := em.Get(i); ok {
			t.Fatalf("got %v, %v - expected %v, %v", v, ok, nil, false)
		}
	}
}

func TestExpireMap_StoppedAndClose(t *testing.T) {
	em := New()
	defer em.Close()
	if stopped := em.Stopped(); stopped {
		t.Fatal("did not call Close(), but it is stopped")
	}
	em.Close()
	if stopped := em.Stopped(); !stopped {
		t.Fatal("called Close(), but it is not stopped")
	}
	em.Close()
	em.Close()
	em.Close()
}

func TestExpireMap_GetAll(t *testing.T) {
	em := New()
	defer em.Close()
	// Test 1 - Test basic
	var ttl1, ttl2 time.Duration = 1, 5

	em.now = func() int64 { return 0 }

	for i := 0; i < 1000; i++ {
		if i%2 == 1 {
			em.Set(i, i, ttl1)
		} else {
			em.Set(i, i, ttl2)
		}
	}

	kvs := em.GetAll()

	if len(kvs) != 1000 {
		t.Fatalf("got %v, expected %v", len(kvs), 1000)
	}

	slices.SortFunc(kvs, func(a, b KeyValue) int {
		return a.Key.(int) - b.Key.(int)
	})

	for i, kv := range kvs {
		if kv.Key != i || kv.Value != i {
			t.Fatal("slice does not contain expected keys and values")
		}
	}
	em.now = func() int64 { return 2 }

	kvs = em.GetAll()

	if len(kvs) != 500 {
		t.Fatalf("got %v, want %v", len(kvs), 500)
		return
	}

	slices.SortFunc(kvs, func(a, b KeyValue) int {
		return a.Key.(int) - b.Key.(int)
	})

	for i, kv := range kvs {
		if kv.Key != i*2 || kv.Value != i*2 {
			t.Fatal("slice does not contain expected values")
			return
		}
	}
}

func TestExpireMap_Notify(t *testing.T) {
	em := New()
	defer em.Close()

	em.now = func() int64 { return 0 }

	c := make(chan Event, 10)

	em.Notify(c, AllEvents)
	em.Set(1, 1, 1)

	if e := <-c; e.Key != 1 || e.Value != 1 || e.Type != Set {
		t.Fatalf("got %v, %v, %v - expected %v, %v, %v", e.Key, e.Value, e.Type, 1, 1, Set)
	}

	em.Set(1, 2, 2)

	if e := <-c; e.Key != 1 || e.Value != 2 || e.Type != Update {
		t.Fatalf("got %v, %v, %v - expected %v, %v, %v", e.Key, e.Value, e.Type, 1, 2, Update)
	}

	em.Set(2, 2, 1)

	if e := <-c; e.Key != 2 || e.Value != 2 || e.Type != Set {
		t.Fatalf("got %v, %v, %v - expected %v, %v, %v", e.Key, e.Value, e.Type, 2, 2, Set)
	}

	em.Delete(2)

	if e := <-c; e.Key != 2 || e.Value != 2 || e.Type != Delete {
		t.Fatalf("got %v, %v, %v - expected %v, %v, %v", e.Key, e.Value, e.Type, 2, 2, Delete)
	}

	em.now = func() int64 { return 2 }

	if e := <-c; e.Key != 1 || e.Value != 2 || e.Type != Expire {
		t.Fatalf("got %v, %v, %v - expected %v, %v, %v", e.Key, e.Value, e.Type, 1, 2, Expire)
	}
}
