package lockqueue

import (
	"sync"
	"sync/atomic"
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
	tail refType = iota // tail is a non-last segment in the path
	head refType = iota // head is the last segment of the path
)

type lockRef struct {
	t refType
	r *dagLock.Vertex
}

// LockSpace allows you to acquire an atomic lock for a set of ResourceLocks.
// Use NewLockSpaceRun() to create a new LockSpace and Stop() to finish the goroutines it spawns.
type LockSpace struct {
	mx              sync.Mutex
	segmentTokens   setCounter.SetCounter
	lockSurface     map[string][]lockRef
	garbage         chan [][]tokenRef
	rootRef         tokenRef
	activeLockers   *sync.WaitGroup
	cleanerFinished chan struct{}
	closed          int32
	statistics      Statistics
}

type tokenRef uintptr

func NewLockSpaceRun() *LockSpace {
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

// Stop forbids locking new groups and returns when the cleaner finishes with the remaining garbage.
func (ls *LockSpace) Stop() {
	if atomic.AddInt32(&ls.closed, 1) > 1 {
		panic("LockSpace is already closed")
	}

	ls.activeLockers.Wait()

	close(ls.garbage)

	<-ls.cleanerFinished
}

// Statistics represents current state of LockSpace.
type Statistics struct {
	GroupsPending int64 // number of groups waiting for acquiring locks for all their resources.
	GroupsLocked  int64 // number of groups waiting acquired locks for all their resources.
	TokenCount    int64 // number of tokens (parts of a path) being stored
	VertexCount   int64 // number of vertices (parts of a path) being stored
	PathCount     int64 // number of unique paths requested. There is a refStack with lockRefs for each path.
	LockrefCount  int64 // number of references to vertexes being stored inside refStacks
}

// LockGroup is used to lock a group of resourceLock's.
// You may pass you own unlocker as the second argument (unlock) and use it to unlock the group.
// The returned value can be used to receive the reference to the second argument (unlock) if provided.
// If unlock is not provided, it is made internally. This is the preferred way to ensure you won't unlock the group before it acquires the lock.
// It is safe to call LockGroup multiple times.
func (ls *LockSpace) LockGroup(lockGroup []ResourceLock, unlocker ...Unlocker) GroupLocker {
	ls.activeLockers.Add(1)

	if atomic.LoadInt32(&ls.closed) > 0 {
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

	ls.statistics.GroupsPending++

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

			// Do nothing if the group aready has a head in the stack.
			lastRef := refStack[len(refStack)-1]
			if lastRef.t == head && groupVertexes.Has(lastRef.r) {
				break
			}

			if refType == head {
				vertexBound := false
				for i := len(refStack) - 1; i >= 0; i-- {
					// Do not bind to siblings' tails.
					if groupVertexes.Has(refStack[i].r) {
						continue
					}

					if refStack[i].t == head {
						if vertexBound {
							break
						}

						vertexBound = true
						refStack[i].r.AddChild(vertex)
						break
					}

					vertexBound = true
					refStack[i].r.AddChild(vertex)
				}

				// Substitute all refs with the new head
				ls.lockSurface[path] = []lockRef{{t: refType, r: vertex}}

				continue
			}

			// Check if the group has left a tail in the stack.
			groupLocked := false
			for _, ref := range refStack {
				if groupVertexes.Has(ref.r) {
					groupLocked = true
					break
				}
			}

			// If the group has left a tail in the stack, do nothing.
			if groupLocked {
				continue
			}

			// If there is a head in the stack, bind to it.
			for i := len(refStack) - 1; i >= 0; i-- {
				if refStack[i].t == head {
					refStack[i].r.AddChild(vertex)
					break
				}
			}

			// Otherwise, just left a tail in the stack.
			ls.lockSurface[path] = append(refStack, lockRef{t: refType, r: vertex})
		}
	}

	ls.mx.Unlock()

	go func() {
		ch := <-u.ch

		ls.mx.Lock()

		for _, v := range vertexes {
			v.Unlock()
		}

		close(ch)

		for _, l := range lockGroup {
			ls.releaseTokens(l.Path)
		}

		ls.mx.Unlock()

		ls.garbage <- tokenRefGroup

		ls.statistics.GroupsLocked--

		ls.activeLockers.Done()
	}()

	lockWaiter := GroupLocker{
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

				ls.statistics.GroupsPending--
				ls.statistics.GroupsLocked++
			}()

			return lockWaiter
		}
	}

	lockWaiter.makeReady(u)

	ls.statistics.GroupsPending--
	ls.statistics.GroupsLocked++

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
