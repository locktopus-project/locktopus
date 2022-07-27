package internal

import (
	"sync"
	"time"
	"unsafe"

	dagLock "github.com/xshkut/distributed-lock/pgk/dag_lock"
	"github.com/xshkut/distributed-lock/pgk/set"
	setCounter "github.com/xshkut/distributed-lock/pgk/set_counter"
)

const garbageBufferSize = 10000
const surfaceCleaningBufferingMs = 1000

type LockType = dagLock.LockType

const LockTypeRead LockType = dagLock.LockTypeRead
const LockTypeWrite LockType = dagLock.LockTypeWrite

type ResourceLock struct {
	lockType LockType
	path     []string
}

func NewResourceLock(lockType dagLock.LockType, path []string) ResourceLock {
	return ResourceLock{
		lockType: lockType,
		path:     path,
	}
}

type refType int8

const (
	tail refType = iota
	head refType = iota
)

type lockRef struct {
	t refType
	r *dagLock.Vertex
}

type LockSpace struct {
	mx              sync.Mutex
	segmentRegistry setCounter.SetCounter
	lockSurface     map[string][]lockRef
	rootRef         segmentRef
	surfaceGarbage  chan [][]segmentRef
}

type segmentRef uintptr

func NewLockSpace() *LockSpace {
	ls := LockSpace{
		segmentRegistry: setCounter.NewSetCounter(),
		lockSurface:     make(map[string][]lockRef),
		surfaceGarbage:  make(chan [][]segmentRef, garbageBufferSize),
	}

	ls.rootRef = ls.storeSegment("root")

	go cleanLockSurfaceGarbage(&ls)

	return &ls
}

type Lock struct {
	ch chan Unlocker
	u  Unlocker
}

// Acquire returns when the lock is acquired.
// You may think of it as the casual method Lock from sync.Mutex.
// The reason why the name differs is that the lock actually starts its lifecycle within LockGroup call.
// Use the returned value to unlock the group.
func (l Lock) Acquire() Unlocker {
	<-l.ch

	return l.u
}

func (l Lock) makeReady(u Unlocker) {
	l.u = u
	l.ch <- l.u
	close(l.ch)
}

type Unlocker struct {
	ch chan struct{}
}

func NewUnlocker() Unlocker {
	return Unlocker{
		ch: make(chan struct{}),
	}
}

func (u Unlocker) Unlock() {
	close(u.ch)
}

// LockGroup is used to lock a group of resourceLock's.
// You may pass you own unlocker as the second argument (unlock) and use it to unlock the group.
// The returned value can be used to receive the reference to the second argument (unlock) if provided.
// If unlock is not provided, it is made internally. This is the preferred way to ensure you won't unlock the group before it acquires the lock.
// It is safe to call LockGroup multiple times.
func (ls *LockSpace) LockGroup(group []ResourceLock, unlocker ...Unlocker) Lock {
	vertexes := make([]*dagLock.Vertex, len(group))
	var u Unlocker

	if len(unlocker) > 1 {
		panic("Passed more than one unlocker. Review your logic")
	} else if len(unlocker) == 1 {
		u = unlocker[0]
	} else {
		u = NewUnlocker()
	}

	ls.mx.Lock()

	segmentGroup := make([][]segmentRef, len(group))

	for i, record := range group {
		segmentGroup[i] = append([]segmentRef{ls.rootRef}, ls.storeSegments(record.path)...)
	}

	groupVertexes := make(set.Set[*dagLock.Vertex])

	for i, segmentRefs := range segmentGroup {
		vertex := dagLock.NewVertex(group[i].lockType)
		vertexes[i] = vertex
		groupVertexes.Add(vertex)

		for i := range segmentRefs {
			path := concatSegmentRefs(segmentRefs[:i+1])

			refType := tail
			if i == len(segmentRefs)-1 {
				refType = head
			}

			existingRefs, ok := ls.lockSurface[path]

			if !ok {
				ls.lockSurface[path] = []lockRef{{t: refType, r: vertex}}
				continue
			}

			if refType == head {
				bound := false
				groupBound := false
				for i := len(existingRefs) - 1; i >= 0; i-- {
					if groupVertexes.Has(existingRefs[i].r) {
						groupBound = true
						continue
					}

					if existingRefs[i].t == head && bound {
						break
					}

					existingRefs[i].r.AddChild(vertex)
					bound = true
				}

				if !groupBound {
					ls.lockSurface[path] = []lockRef{{t: refType, r: vertex}}
				}

				continue
			}

			groupHasBounding := false
			for _, existingRef := range existingRefs {
				if groupVertexes.Has(existingRef.r) {
					groupHasBounding = true
					break
				}
			}

			if groupHasBounding {
				continue
			}

			for i := len(existingRefs) - 1; i >= 0; i-- {
				if existingRefs[i].t == head {
					existingRefs[i].r.AddChild(vertex)
					break
				}
			}

			ls.lockSurface[path] = append(existingRefs, lockRef{t: refType, r: vertex})
		}
	}

	go func() {
		<-u.ch

		ls.mx.Lock()

		for _, v := range vertexes {
			v.Unlock()
		}

		for _, l := range group {
			ls.releaseSegments(l.path)
		}

		ls.mx.Unlock()

		ls.surfaceGarbage <- segmentGroup
	}()

	ls.mx.Unlock()

	lockWaiter := Lock{
		u:  u,
		ch: make(chan Unlocker, 1),
	}

	for i, v := range vertexes {
		vertexLock := v.LockChan()

		select {
		case <-vertexLock:
			continue
		default:
			go func() {
				<-vertexLock
				for _, v := range vertexes[i+1:] {
					v.Lock()
				}

				lockWaiter.makeReady(u)
			}()

			return lockWaiter
		}

	}

	lockWaiter.makeReady(u)
	return lockWaiter
}

func (ls *LockSpace) storeSegment(segment string) segmentRef {
	p := ls.segmentRegistry.Store(segment)

	return segmentRef(unsafe.Pointer(p))
}

func (ls *LockSpace) storeSegments(segments []string) []segmentRef {
	refs := make([]segmentRef, len(segments))

	for i, s := range segments {
		refs[i] = ls.storeSegment(s)
	}

	return refs
}

func (ls *LockSpace) releaseSegment(segment string) {
	ls.segmentRegistry.Release(segment)
}

func (ls *LockSpace) releaseSegments(segments []string) {
	for _, s := range segments {
		ls.releaseSegment(s)
	}
}

func cleanLockSurfaceGarbage(ls *LockSpace) {
	for {
		segmentGroups := make([][][]segmentRef, 0, 1)

		ok := true
		for ok {
			select {
			case segmentGroup := <-ls.surfaceGarbage:
				segmentGroups = append(segmentGroups, segmentGroup)
				ok = true
			default:
				ok = false
			}
		}

		if len(segmentGroups) == 0 {
			time.Sleep(surfaceCleaningBufferingMs * time.Millisecond)
			continue
		}

		paths := make(set.Set[string])

		for _, segmentGroup := range segmentGroups {
			for _, segmentRefs := range segmentGroup {
				for i := range segmentRefs {
					paths.Add(concatSegmentRefs(segmentRefs[:i+1]))
				}
			}
		}

		ls.mx.Lock()

		for path := range paths {
			segmentRefs, ok := ls.lockSurface[path]
			if !ok {
				continue
			}

			keepFrom := 0

			for i, segmentRef := range segmentRefs {
				if !segmentRef.r.Useless() {
					break
				}

				keepFrom = i + 1
			}

			if keepFrom == 0 {
				continue
			}

			if keepFrom == len(segmentRefs) {
				delete(ls.lockSurface, path)
				continue
			}

			ls.lockSurface[path] = segmentRefs[keepFrom:]
		}

		ls.mx.Unlock()

		time.Sleep(surfaceCleaningBufferingMs * time.Millisecond)
	}
}
