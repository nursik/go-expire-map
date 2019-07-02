# Go expire map
[![GoDoc](https://godoc.org/github.com/nursik/go-expire-map?status.svg)](https://godoc.org/github.com/nursik/go-expire-map)

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
    ttl := time.Unix(1735689600, 0)
    
    // Insert
    expireMap.SetEx(1, 1, ttl)

    // Get value
    v, ok := expireMap.Get(1)
    fmt.Println(v, ok)
    // Output 1 true

    // Get TTL
    v = expireMap.TTL(1)
    fmt.Println(v)
    // Output 1735689600000000000

    // Update TTL
    v, ok = expireMap.Expire(1, time.Now().Add(time.Second))
    fmt.Println(v, ok)
    // Output 1 true

    time.Sleep(time.Second + time.Millisecond)

    // Because key is already expired, it returns nil, false
    v, ok = expireMap.Expire(1, ttl)
    fmt.Println(v, ok)
    // Output nil false

    // Because key is already expired, it returns nil, false
    v, ok = expireMap.Get(1)
    fmt.Println(v, ok)
    // Output nil false
}

```

## Benchmarks

Every entry is nanoseconds per operation (benchmark output / N). N is number of keys

| Benchmark\N  | 1000 | 10000 | 100000 | 1000000 | 10000000 |
| ------------ | ---- | ----- | ------ | ------- | -------- |
| **Insert into empty map**     | 281 | 351 | 302 | 504 | 572 |
| **Update values**             | 101 | 109 | 122 | 182 | 201 |
| **Delete keys**               | 132 | 134 | 151 | 214 | 233 |
| **Update TTL**                |  68 | 73  | 83  | 142 | 157 |
| **Remove key using Expire()** | 136 | 137 | 156 | 217 | 237 |

Benchmarks for Get(), when there are already expired keys

| Expired keys\N  | 1000 | 10000 | 100000 | 1000000 | 10000000 |
| ------------    | ---- | ----- | ------ | ------- | -------- |
| **50%**         | 308  | 285   | 163    | 205     | 220 |
| **33%**         | 295  | 225   | 136    | 184     | 198 |
| **25%**         | 264  | 221   | 125    | 175     | 188 |
| **20%**         | 270  | 220   | 121    | 165     | 181 |
| **10%**         | 223  | 170   | 101    | 152     | 164 |
| **0%**          | 185  | 146   |  85    | 133     | 147 | 

There are also other benchmarks see [benchmark.log](https://github.com/nursik/go-expire-map/blob/master/benchmark.log)