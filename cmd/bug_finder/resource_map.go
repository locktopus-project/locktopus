package main

import (
	"sync"

	ml "github.com/xshkut/gearlock/pgk/multilocker"
)

type resourceRef struct {
	group     *int8
	t         ml.LockType
	resources []ml.ResourceLock
}

type ResourceMap struct {
	sync.RWMutex
	items map[string]resourceRef
}

func NewResourceMap() *ResourceMap {
	return &ResourceMap{
		items: make(map[string]resourceRef),
	}
}

func (m *ResourceMap) Get(key string) (resourceRef, bool) {
	m.RLock()
	defer m.RUnlock()

	r, ok := m.items[key]
	return r, ok
}

func (m *ResourceMap) Set(key string, r resourceRef) {
	m.Lock()
	defer m.Unlock()

	m.items[key] = r
}

func (m *ResourceMap) Delete(key string) {
	m.Lock()
	defer m.Unlock()

	delete(m.items, key)
}
