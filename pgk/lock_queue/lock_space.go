package internal

import (
	"sync"
	"time"
	"unsafe"

	dagLock "github.com/xshkut/distributed-lock/pgk/dag_lock"
	"github.com/xshkut/distributed-lock/pgk/set"
	setCounter "github.com/xshkut/distributed-lock/pgk/set_counter"
)

type LockType = dagLock.LockType

const LockTypeRead LockType = dagLock.LockTypeRead
const LockTypeWrite LockType = dagLock.LockTypeWrite

const garbageBufferSize = 10000
const surfaceCleaningBufferingMs = 1000

type ResourceLock struct {
	LockType LockType
	Path     []string
}

func NewResourceLock(lockType LockType, path []string) ResourceLock {
	return ResourceLock{
		LockType: lockType,
		Path:     path,
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

// LockSpace
type LockSpace struct {
	mx            sync.Mutex
	segmentTokens setCounter.SetCounter
	lockSurface   map[string][]lockRef
	garbage       chan [][]tokenRef
	rootRef       tokenRef
}

type tokenRef uintptr

func NewLockSpace() *LockSpace {
	ls := LockSpace{
		segmentTokens: setCounter.NewSetCounter(),
		lockSurface:   make(map[string][]lockRef),
		garbage:       make(chan [][]tokenRef, garbageBufferSize),
	}

	ls.rootRef = ls.storeTokens([]string{""})[0]

	go cleanRefStacks(&ls)

	return &ls
}

func (ls *LockSpace) Destroy() {
	close(ls.garbage)
}

// type Statistics struct {
// 	groupsPending int64
// 	groupsLocked  int64
// 	tokenCount    int64
// 	vertexCount   int64
// 	pathCount     int64
// }

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
func (ls *LockSpace) LockGroup(lockGroup []ResourceLock, unlocker ...Unlocker) Lock {
	vertexes := make([]*dagLock.Vertex, len(lockGroup))
	var u Unlocker

	if len(unlocker) > 1 {
		panic("Passed more than one unlocker. Review your logic")
	} else if len(unlocker) == 1 {
		u = unlocker[0]
	} else {
		u = NewUnlocker()
	}

	ls.mx.Lock()

	tokenRefGroup := make([][]tokenRef, len(lockGroup))

	for i, record := range lockGroup {
		tokenRefGroup[i] = append([]tokenRef{ls.rootRef}, ls.storeTokens(record.Path)...)
	}

	groupVertexes := set.NewSet[*dagLock.Vertex]()

	for i, tokenRefs := range tokenRefGroup {
		vertex := dagLock.NewVertex(lockGroup[i].LockType)
		vertexes[i] = vertex
		groupVertexes.Add(vertex)

		for i := range tokenRefs {
			path := concatSegmentRefs(tokenRefs[:i+1])

			refType := tail
			if i == len(tokenRefs)-1 {
				refType = head
			}

			refsStack, ok := ls.lockSurface[path]

			if !ok {
				ls.lockSurface[path] = []lockRef{{t: refType, r: vertex}}
				continue
			}

			if refType == head {
				vertexBound := false
				groupLocked := false
				for i := len(refsStack) - 1; i >= 0; i-- {
					if groupVertexes.Has(refsStack[i].r) {
						groupLocked = true
						continue
					}

					if refsStack[i].t == head && vertexBound {
						break
					}

					refsStack[i].r.AddChild(vertex)
					vertexBound = true
				}

				if !groupLocked {
					ls.lockSurface[path] = []lockRef{{t: refType, r: vertex}}
				}

				continue
			}

			groupLocked := false
			for _, ref := range refsStack {
				if groupVertexes.Has(ref.r) {
					groupLocked = true
					break
				}
			}

			if groupLocked {
				continue
			}

			for i := len(refsStack) - 1; i >= 0; i-- {
				if refsStack[i].t == head {
					refsStack[i].r.AddChild(vertex)
					break
				}
			}

			ls.lockSurface[path] = append(refsStack, lockRef{t: refType, r: vertex})
		}
	}

	ls.mx.Unlock()

	go func() {
		<-u.ch

		ls.mx.Lock()

		for _, v := range vertexes {
			v.Unlock()
		}

		for _, l := range lockGroup {
			ls.releaseTokens(l.Path)
		}

		ls.mx.Unlock()

		ls.garbage <- tokenRefGroup
	}()

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

func (ls *LockSpace) storeTokens(segments []string) []tokenRef {
	refs := make([]tokenRef, len(segments))

	for i, s := range segments {
		p := ls.segmentTokens.Store(s)

		refs[i] = tokenRef(unsafe.Pointer(p))
	}

	return refs
}

func (ls *LockSpace) releaseTokens(segments []string) {
	for _, s := range segments {
		ls.segmentTokens.Release(s)
	}
}

func cleanRefStacks(ls *LockSpace) {
	for {
		segmentGroupList := make([][][]tokenRef, 0, 1)

		ok := true
		for ok {
			select {
			case segmentGroup, closed := <-ls.garbage:
				if closed {
					break
				}
				segmentGroupList = append(segmentGroupList, segmentGroup)
				ok = true
			default:
				ok = false
			}
		}

		if len(segmentGroupList) == 0 {
			time.Sleep(surfaceCleaningBufferingMs * time.Millisecond)
			continue
		}

		paths := make(set.Set[string])

		for _, segmentGroup := range segmentGroupList {
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
