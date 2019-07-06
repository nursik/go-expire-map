package expiremap_test

import (
	"fmt"
	expiremap "github.com/nursik/go-expire-map"
	"time"
)

func ExampleExpireMap_Notify() {
	expireMap := expiremap.New()
	defer expireMap.Close()

	c := make(chan expiremap.Event, 10)
	expireMap.Notify(c, expiremap.AllEvents)
	var event expiremap.Event
	// Set
	expireMap.Set("key1", "value1", time.Second)
	event = <-c
	fmt.Println(event.Key, event.Value, event.Type == expiremap.Set)

	// Update
	expireMap.Set("key1", "value1", time.Second)
	event = <-c
	fmt.Println(event.Key, event.Value, event.Type == expiremap.Update)

	expireMap.Set("key2", "value2", time.Hour)
	_ = <-c
	expireMap.Delete("key2")
	event = <-c
	fmt.Println(event.Key, event.Value, event.Type == expiremap.Delete)

	// Causes Expire event to be fired
	time.Sleep(time.Second)

	event = <-c
	fmt.Println(event.Key, event.Value, event.Type == expiremap.Expire)

	// Output:
	// key1 value1 true
	// key1 value1 true
	// key2 value2 true
	// key1 value1 true
}
