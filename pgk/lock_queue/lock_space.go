package lockqueue

import (
	"sync"
	"unsafe"

	dagLock "github.com/xshkut/distributed-lock/pgk/dag_lock"
	"github.com/xshkut/distributed-lock/pgk/set"
	setCounter "github.com/xshkut/distributed-lock/pgk/set_counter"
)

type LockType = dagLock.LockType

const LockTypeRead LockType = dagLock.LockTypeRead
const LockTypeWrite LockType = dagLock.LockTypeWrite

const garbageBufferSize = 100

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

// LockSpace allows you to acquire an atomic lock for a set of ResourceLocks.
// Use CreateAndRun() to create a new LockSpace and run its garbage collector. Stop() will stop the garbage collector.
type LockSpace struct {
	mx              sync.Mutex
	segmentTokens   setCounter.SetCounter
	lockSurface     map[string][]lockRef
	garbage         chan [][]tokenRef
	rootRef         tokenRef
	activeLockers   *sync.WaitGroup
	cleanerFinished chan struct{}
	closed          bool
}

type tokenRef uintptr

func CreateAndRun() *LockSpace {
	ls := LockSpace{
		segmentTokens:   setCounter.NewSetCounter(),
		lockSurface:     make(map[string][]lockRef),
		garbage:         make(chan [][]tokenRef, garbageBufferSize),
		activeLockers:   &sync.WaitGroup{},
		cleanerFinished: make(chan struct{}),
	}

	ls.rootRef = ls.storeTokens([]string{""})[0]

	go ls.cleanRefStacks()

	return &ls
}

func (ls *LockSpace) Stop() {
	if ls.closed {
		panic("LockSpace is already closed")
	}

	ls.closed = true

	ls.activeLockers.Wait()

	close(ls.garbage)

	<-ls.cleanerFinished
}

// Statistics represents current state of LockSpace.
// GroupsPending - number of groups waiting for acquiring locks for all their resources.
// GroupsLocked - number of groups waiting acquired locks for all their resources.
// TokenCount - number of tokens (parts of a path) being stored
// vertexCount - number
// type Statistics struct {
// 	groupsPending int64
// 	groupsLocked  int64
// 	tokenCount    int64
// 	vertexCount   int64
// 	pathCount     int64
// }

// LockGroup is used to lock a group of resourceLock's.
// You may pass you own unlocker as the second argument (unlock) and use it to unlock the group.
// The returned value can be used to receive the reference to the second argument (unlock) if provided.
// If unlock is not provided, it is made internally. This is the preferred way to ensure you won't unlock the group before it acquires the lock.
// It is safe to call LockGroup multiple times.
func (ls *LockSpace) LockGroup(lockGroup []ResourceLock, unlocker ...Unlocker) Lock {
	ls.activeLockers.Add(1)

	if ls.closed {
		panic("LockSpace is closed")
	}

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

			refStack, ok := ls.lockSurface[path]

			if !ok {
				ls.lockSurface[path] = []lockRef{{t: refType, r: vertex}}
				continue
			}

			if refType == head {
				vertexBound := false
				groupLocked := false
				for i := len(refStack) - 1; i >= 0; i-- {
					if groupVertexes.Has(refStack[i].r) {
						groupLocked = true
						continue
					}

					if refStack[i].t == head && vertexBound {
						break
					}

					refStack[i].r.AddChild(vertex)
					vertexBound = true
				}

				if !groupLocked {
					ls.lockSurface[path] = []lockRef{{t: refType, r: vertex}}
				}

				continue
			}

			groupLocked := false
			for _, ref := range refStack {
				if groupVertexes.Has(ref.r) {
					groupLocked = true
					break
				}
			}

			if groupLocked {
				continue
			}

			for i := len(refStack) - 1; i >= 0; i-- {
				if refStack[i].t == head {
					refStack[i].r.AddChild(vertex)
					break
				}
			}

			ls.lockSurface[path] = append(refStack, lockRef{t: refType, r: vertex})
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

		ls.activeLockers.Done()
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

func (ls *LockSpace) cleanRefStacks() {
	var opened bool

	for opened {
		segmentGroupList := make([][][]tokenRef, 0, 1)
		var segmentGroup [][]tokenRef

		segmentGroup, opened = <-ls.garbage
		if !opened {
			break
		}

		segmentGroupList = append(segmentGroupList, segmentGroup)

		for !opened {
			select {
			case segmentGroup, opened = <-ls.garbage:
				if opened {
					segmentGroupList = append(segmentGroupList, segmentGroup)
				}
			default:
			}
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
			refStack, ok := ls.lockSurface[path]
			if !ok {
				continue
			}

			keepFrom := 0

			for i, segmentRef := range refStack {
				if !segmentRef.r.Useless() {
					break
				}

				keepFrom = i + 1
			}

			if keepFrom == 0 {
				continue
			}

			if keepFrom == len(refStack) {
				delete(ls.lockSurface, path)
				continue
			}

			ls.lockSurface[path] = refStack[keepFrom:]
		}

		ls.mx.Unlock()
	}

	ls.cleanerFinished <- struct{}{}
}
