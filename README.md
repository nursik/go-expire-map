# Go expire map
[![GoDoc](https://godoc.org/github.com/nursik/go-expire-map?status.svg)](https://godoc.org/github.com/nursik/go-expire-map)
[![Go Report Card](https://goreportcard.com/badge/github.com/nursik/go-expire-map)](https://goreportcard.com/report/github.com/nursik/go-expire-map)

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

## Custom expire maps
Expire map uses `interface{}` as a type for keys and values, but you can generate a custom map with different types.
Benchmarks showed that it can be 25% faster than current implementation (benchmarks were done on `struct{x int, y string, z string}` as key and `struct{x int, y string, z string, t string}` as value).

To generate a map with different types use `gen/gen.go`. It prints a code to stdout. You must save output to a file and fix imports, if necessary. Also, it uses `github.com/nursik/go-ordered-set` and you need import it too.

`gen.go` assumes that value is a struct and that's why you don't need to set zero-value argument (-zv). But if you need other types as a value than struct, you must manually set zero-value.
Default type for key and value is `interface{}`. Also you can change default name of the map struct (ExpireMap) using -mp argument.

Example:
```
// Generate map with Animal{} as key and Planet{} as value
go run gen.go -k Animal -v Planet

// Generate map with Animal{} as key, *Planet{} as value and CustomMap as a map name 
go run gen.go -k Animal -v *Planet -mp CustomMap

// Generate map with int as key and *Planet{} as value
go run gen.go -k int -v *Planet

// Generate map with int as key and string as value
go run gen.go -k int -v string -zv "\"\""

// Generate map with interface as key and []int as value
go run gen.go -v "[]int" -zv "nil"

```
## Benchmarks
Benchmarks are done on Asus ROG GL553V with:
* CPU: Intel® Core™ i7-7700HQ CPU @ 2.80GHz × 8
* RAM: DDR4 2400 SDRAM, 8 GB

Every entry is nanoseconds per operation (benchmark output / N). N is number of keys

| Benchmark\N  | 1000 | 10000 | 100000 | 1000000 | 10000000 |
| ------------ | ---- | ----- | ------ | ------- | -------- |
| **Insert into empty map**     | 281 | 351 | 302 | 504 | 572 |
| **Update values**             | 101 | 109 | 122 | 182 | 201 |
| **Delete keys**               | 132 | 134 | 151 | 214 | 233 |
| **Update TTL**                |  68 | 73  | 83  | 142 | 157 |
| **Remove key using SetTTL()** | 136 | 137 | 156 | 217 | 237 |

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