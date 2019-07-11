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

To generate a map with different types use `gen/gen.go`. It prints a code to stdout. You must save output to a file and fix imports, if necessary. Also, it uses `github.com/nursik/go-ordered-set` and you need to import it too.

You can check that a generated map works properly by generating tests using `gen/tests_gen.go`. It works in the same way as `gen.go`.

`gen.go` assumes that value is a struct and that's why you don't need to set zero-value argument (-zv). But if you need other types as a value than struct, you must manually set zero-value.
Default type for key and value is `interface{}`. Also you can change default name of the map struct (ExpireMap) using -mp argument.

If you don't need notifications, you may remove this feature adding `-n=false` to arguments. A map with no notifications is a little bit faster (around 20ns faster for `SetTTL()`).

Example:
```
// Generate a map with Animal{} as key and Planet{} as value
go run gen.go -k Animal -v Planet

// Generate a map with Animal{} as key, *Planet{} as value and CustomMap as a map name 
go run gen.go -k Animal -v *Planet -mp CustomMap

// Generate a map with int as key and *Planet{} as value
go run gen.go -k int -v *Planet

// Generate a map with int as key and string as value
go run gen.go -k int -v string -zv "\"\""

// Generate a map with interface as key and []int as value
go run gen.go -v "[]int" -zv "nil"

// Generate a map with int as a key, interface as a value and with no notifications feature
go run gen.go -k int -n=false

```
## Benchmarks
Benchmarks are done on Asus ROG GL553V with:
* CPU: Intel® Core™ i7-7700HQ CPU @ 2.80GHz × 8
* RAM: DDR4 2400 SDRAM, 8 GB

Every entry is nanoseconds per operation (benchmark output / N). N is number of keys

#### Without notifications channel set
| Benchmark\N  | 1000 | 10000 | 100000 | 1000000 | 10000000 |
| ------------ | ---- | ----- | ------ | ------- | -------- |
| **Insert into empty map**     | 280 | 348 | 303 | 504 | 561 |
| **Update values**             | 100 | 107 | 120 | 179 | 199 |
| **Delete keys**               | 160 | 154 | 189 | 241 | 256 |
| **Update TTL**                | 86  | 92  | 103 | 162 | 174 |
| **Remove key using SetTTL()** | 164 | 158 | 192 | 239 | 257 |

Benchmarks for Get(), when there are already expired keys

| Expired keys\N  | 1000 | 10000 | 100000 | 1000000 | 10000000 |
| ------------    | ---- | ----- | ------ | ------- | -------- |
| **50%**         | 341  | 311   | 193    | 228     | 237 |
| **33%**         | 330  | 294   | 177    | 205     | 218 |
| **25%**         | 297  | 266   | 156    | 188     | 203 |
| **20%**         | 325  | 268   | 155    | 184     | 198 |
| **10%**         | 265  | 223   | 138    | 169     | 181 |
| **0%**          | 233  | 182   | 121    | 150     | 162 | 


#### With notifications channel set
| Benchmark\N  | 1000 | 10000 | 100000 | 1000000 | 10000000 |
| ------------ | ---- | ----- | ------ | ------- | -------- |
| **Insert into empty map**     | 451 | 510 | 454 | 698 | 737 |
| **Update values**             | 258 | 274 | 280 | 345 | 345 |
| **Delete keys**               | 312 | 312 | 367 | 409 | 448 |
| **Update TTL**                | 240 | 269 | 276 | 339 | 344 |
| **Remove key using SetTTL()** | 336 | 331 | 364 | 416 | 432 |

Benchmarks for Get(), when there are already expired keys

| Expired keys\N  | 1000 | 10000 | 100000 | 1000000 | 10000000 |
| ------------    | ---- | ----- | ------ | ------- | -------- |
| **50%**         | 260  | 266   | 303    | 349     | 353 |
| **33%**         | 212  | 224   | 243    | 301     | 331 |
| **25%**         | 185  | 193   | 221    | 260     | 274 |
| **20%**         | 171  | 180   | 204    | 251     | 264 |
| **10%**         | 139  | 149   | 173    | 228     | 259 |
| **0%**          | 107  | 111   | 128    | 184     | 188 | 


There are also other benchmarks see [benchmark.log](https://github.com/nursik/go-expire-map/blob/master/benchmark.log)
and [benchmark_with_channel.log](https://github.com/nursik/go-expire-map/blob/master/benchmark_with_channel.log)