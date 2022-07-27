package internal

import (
	"sync/atomic"
)

// Set for storing keys. Thread-unsafe.
// Unlike the classic Set, SetCounter counts the balance of Store/Release calls for each key and does not allow to release unexisting keys.
type SetCounter struct {
	m map[string]*int64
}

func NewSetCounter() SetCounter {
	return SetCounter{
		m: map[string]*int64{},
	}
}

func (tm *SetCounter) Get(key string) *int64 {
	value := tm.m[key]

	return value
}

func (tm *SetCounter) Store(key string) (value *int64) {
	var exists bool

	value, exists = tm.m[key]
	if !exists {
		value = new(int64)
		tm.m[key] = value
	}

	atomic.AddInt64(value, 1)

	return value
}

func (tm *SetCounter) Release(key string) *int64 {
	value, ok := tm.m[key]
	if !ok {
		panic("Attempt to release non-existing key. Please, review your code.")
	}

	atomic.AddInt64(value, -1)

	if atomic.LoadInt64(value) == 0 {
		delete(tm.m, key)

		return nil
	}

	return value
}
