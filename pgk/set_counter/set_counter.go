package internal

import (
	"sync/atomic"
)

// Set for storing keys. Thread-unsafe.
// Unlike the classic Set, SetCounter counts the balance of Store/Release calls for each key and does not allow to release unexisting keys.
// It returns pointers so they can be used as unique attributes of keys.
// SetCounter is not thread-safe.
type SetCounter struct {
	m     map[string]*int64
	count int64
}

func NewSetCounter() SetCounter {
	return SetCounter{
		m:     map[string]*int64{},
		count: 0,
	}
}

func (sc *SetCounter) Get(key string) *int64 {
	value := sc.m[key]

	return value
}

func (sc *SetCounter) Store(key string) (value *int64) {
	var exists bool

	value, exists = sc.m[key]
	if !exists {
		value = new(int64)
		sc.m[key] = value
	}

	atomic.AddInt64(value, 1)

	sc.count++

	return value
}

func (sc *SetCounter) Release(key string) *int64 {
	value, ok := sc.m[key]
	if !ok {
		panic("Attempt to release non-existing key. Please, review your code.")
	}

	atomic.AddInt64(value, -1)

	sc.count--

	if atomic.LoadInt64(value) == 0 {
		delete(sc.m, key)

		return nil
	}

	return value
}

// Sum returns the sum of all stored values (non-unique).
func (sc *SetCounter) Sum() int64 {
	return sc.count
}

// Count returns the number of unique keys in the set.
func (sc *SetCounter) Count() int64 {
	return int64(len(sc.m))
}
