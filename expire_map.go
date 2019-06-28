package expiremap

import "time"

const timeResolution = time.Millisecond

type KeyValue struct {
	key interface{}
	value interface{}
}

type ExpireMap interface {
	// Expire sets a new ttl for the key. If due variable is less than current time,
	// it removes key from the map and returns false. If key is expired, it is
	// removed from the map and false wiil be returned. If key does not exist, it
	// returns false. Finally, if key exists and not expired, it's ttl is updated and
	// true wiil be returned
	Expire(key interface{}, due time.Time) bool
	// Get returns a value and existence flag for the given key. It is guaranteed
	// that if key is expired, it will not be returned (nil and false will be
	// returned)
	Get(key interface{}) (interface{}, bool)
	// Delete removes key from the map
	Delete(key interface{})
	// Close stops internal goroutines, and sets nil to all internal variables
	Close()
	// SetEx sets or updates value and ttl for the given key. If due variable is
	// less than current time, it does nothing
	SetEx(key interface{}, value interface{}, due time.Time)
	// GetAll returns a slice of KeyValue. It guarantees, that at the moment of
	// the method call, all keys in the slice are not expired
	GetAll() []KeyValue
	// Size returns the current size of the map. Current size is equal to
	// the number of keys stored in the map, both expired and unexpired
	Size() int
	// Curtime returns inner curtime variable. Returned value is int64 representing
	// Unix time in nanoseconds
	Curtime() int64
	// Stopped indicates if the map is stopped. You should not call any methods on stopped map
	Stopped() bool
}

