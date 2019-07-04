package expiremap_test

import (
	"fmt"
	"github.com/nursik/go-expire-map"
	"time"
)

func Example() {
	expireMap := expiremap.New()
	// You must call Close(), when you do not need the map anymore
	defer expireMap.Close()

	// GMT Wednesday, 1 January 2025, 0:00:00
	far := time.Unix(1735689600, 0)

	ttl := far.Sub(time.Now())

	// Insert
	expireMap.Set(1, 1, ttl)

	// Get value
	v, ok := expireMap.Get(1)
	fmt.Println(v, ok)
	// Output 1 true

	// Get TTL
	v = expireMap.GetTTL(1)
	// The output is equal to ~ ttl
	fmt.Println(v)

	// Update TTL
	v, ok = expireMap.SetTTL(1, time.Second)
	fmt.Println(v, ok)
	// Output 1 true

	time.Sleep(time.Second + time.Millisecond)

	// Because key is already expired, it returns nil, false
	v, ok = expireMap.SetTTL(1, ttl)
	fmt.Println(v, ok)
	// Output nil false

	// Because key is already expired, it returns nil, false
	v, ok = expireMap.Get(1)
	fmt.Println(v, ok)
	// Output nil false
}
