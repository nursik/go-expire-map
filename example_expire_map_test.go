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
	ttl := time.Unix(1735689600, 0)

	// Insert
	expireMap.Set(1, 1, ttl)

	// Get value
	v, ok := expireMap.Get(1)
	fmt.Println(v, ok)
	// Output 1 true

	// Get TTL
	v = expireMap.GetTTL(1)
	fmt.Println(v)
	// Output 1735689600000000000

	// Update TTL
	v, ok = expireMap.SetTTL(1, time.Now().Add(time.Second))
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
