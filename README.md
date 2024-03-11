# Go expire map
[![GoDoc](https://godoc.org/github.com/nursik/go-expire-map?status.svg)](https://godoc.org/github.com/nursik/go-expire-map)
[![Go Report Card](https://goreportcard.com/badge/github.com/nursik/go-expire-map)](https://goreportcard.com/report/github.com/nursik/go-expire-map)

# Disclaimer!!
This package is considered deprecated as there are more API rich and faster solutions like [ccache](https://github.com/karlseguin/ccache) or [ttlcache](https://github.com/jellydator/ttlcache/blob/v3/cache.go). Also, library uses UnixNano instead of time.Time, which makes it to fail during system time (wall clock) adjustments.
## Quick start

```go
package main

import (
    "fmt"
    "github.com/nursik/go-expire-map"
    "time"
)
func main() {
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

```

## Why/when
* You need thread safe map with `comparable` keys and `interface` values
* You have a lot of inserts with the same TTL. If they all expire at the same time you don't want your app freeze
* You need both active and passive expiration
* You need notifications about inserts, update, deletes and expirations
