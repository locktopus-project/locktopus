package sliceappender

import "sync"

// sliceAppender is a thread-safe wrapper around slice.
type sliceAppender[T any] struct {
	mx sync.Mutex
	s  []T
}

func (as *sliceAppender[T]) Append(elem ...T) {
	as.mx.Lock()
	defer as.mx.Unlock()

	as.s = append(as.s, elem...)
}

func (as *sliceAppender[T]) Value() []T {
	as.mx.Lock()
	defer as.mx.Unlock()

	return as.s
}

func NewSliceAppender[T any]() *sliceAppender[T] {
	as := sliceAppender[T]{}

	as.s = make([]T, 0)

	return &as
}
