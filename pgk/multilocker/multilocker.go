// MultiLocker allows you to acquire an atomic lock for a set of ResourceLocks.
// Use NewMultilocker() to create a new MultiLocker and Close() to finish the goroutines it spawns.

package multilocker

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
const tokenBufferInitialSize = 10

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
	v *dagLock.Vertex
}

type MultiLocker struct {
	mx            sync.Mutex
	segmentTokens setCounter.SetCounter
	lockSurface   map[string][]lockRef
	garbage       chan [][]token
	rootRef       token
	activeLockers *sync.WaitGroup
	cleaned       chan struct{}
	closed        int32
	statistics    statistics
	lastLockID    int64
}

// MultilockerStatistics represents current (non-cumulative) state of MultiLocker
type MultilockerStatistics struct {
	LastGroupID    int64 // Sequence number of the last group (starting from 1)
	GroupsPending  int64 // number of groups waiting for acquiring locks for all their resources
	GroupsAcquired int64 // number of groups waiting acquired locks for all their resources
	LocksPending   int64 // number of resource locks being stored
	LocksAcquired  int64 // number of resource locks being stored
	LockrefCount   int64 // number of references to vertexes being stored in refStacks
	TokensTotal    int64 // number of unique tokens (parts of a path) being stored
	TokensUnique   int64 // number of unique tokens (parts of a path) being stored
	PathCount      int64 // number of unique paths requested. There is a refStack with lockRefs for each path. Initially, MultiLocker has PathCount = 1 (for the root segment)
}

type statistics struct {
	groupsPending       int64
	groupsAcquired      int64
	pendingVertexCount  int64
	acquiredVertexCount int64
	lockrefCount        int64
}

// NewMultilocker creates an instance of MultiLocker. Use Close() to finish its goroutines.
func NewMultilocker() *MultiLocker {
	ls := MultiLocker{
		segmentTokens: setCounter.NewSetCounter(),
		lockSurface:   make(map[string][]lockRef),
		garbage:       make(chan [][]token, garbageBufferSize),
		activeLockers: &sync.WaitGroup{},
		cleaned:       make(chan struct{}),
	}

	ls.rootRef = ls.tokenizeSegments([]string{""})[0]

	go ls.cleanPaths()

	return &ls
}

// Close forbids making new locks and returns when the cleaner finishes with the remaining garbage.
func (ml *MultiLocker) Close() {
	if atomic.AddInt32(&ml.closed, 1) > 1 {
		panic("multilocker is already closed")
	}

	ml.activeLockers.Wait()

	close(ml.garbage)

	<-ml.cleaned
}

func (ml *MultiLocker) Statistics() MultilockerStatistics {
	ml.mx.Lock()
	defer ml.mx.Unlock()

	s := MultilockerStatistics{}

	s.LastGroupID = ml.lastLockID

	s.GroupsPending = atomic.LoadInt64(&ml.statistics.groupsPending)
	s.GroupsAcquired = atomic.LoadInt64(&ml.statistics.groupsAcquired)

	s.LocksPending = atomic.LoadInt64(&ml.statistics.pendingVertexCount)
	s.LocksAcquired = atomic.LoadInt64(&ml.statistics.acquiredVertexCount)

	s.LockrefCount = atomic.LoadInt64(&ml.statistics.lockrefCount)

	s.PathCount = int64(len(ml.lockSurface))
	s.TokensTotal = int64(ml.segmentTokens.Sum())
	s.TokensUnique = int64(ml.segmentTokens.Count())

	return s
}

// Lock is used to atomically lock a slice of ResourceLock's.
// The lock will be acquired as soon as there are no precedent resources whose locks interfere with this ones by path and lock types.
// If unlocker is not provided, it is made internally. In any case, the returned Lock can be used to receive the reference unlocker.
func (ml *MultiLocker) Lock(resourceLocks []ResourceLock, unlocker ...Unlocker) Lock {
	ml.activeLockers.Add(1)

	if atomic.LoadInt32(&ml.closed) > 0 {
		panic("multilocker is closed")
	}

	var u Unlocker

	if len(unlocker) > 1 {
		panic("Passed more than one unlocker. Review your logic")
	} else if len(unlocker) == 1 {
		u = unlocker[0]
	} else {
		u = NewUnlocker()
	}

	locker := ml.lockResources(resourceLocks, u)

	return locker
}

func (ml *MultiLocker) lockResources(lockGroup []ResourceLock, u Unlocker) Lock {
	ml.mx.Lock()

	ml.lastLockID++
	lockID := ml.lastLockID

	atomic.AddInt64(&ml.statistics.groupsPending, 1)

	tokenRefGroup := make([][]token, len(lockGroup))

	for i, record := range lockGroup {
		tokenRefGroup[i] = append([]token{ml.rootRef}, ml.tokenizeSegments(record.Path)...)
	}

	groupVertexes := set.NewSet[*dagLock.Vertex]()

	buffer := newTokenBuffer(tokenBufferInitialSize)

	for i, tokenRefs := range tokenRefGroup {
		lockType := lockGroup[i].LockType
		vertex := dagLock.NewVertex(lockType)
		vAdded := false

		if len(tokenRefs) > len(buffer) {
			buffer = newTokenBuffer(len(tokenRefs) * 2)
		}

	pathIteration: // horizontal iteration over path segments
		for i := range tokenRefs {
			path := concatTokenRefs(tokenRefs[:i+1], buffer)

			refType := tail
			isHead := false
			if i == len(tokenRefs)-1 {
				refType = head
				isHead = true
			}

			existingRefs, ok := ml.lockSurface[path]
			if !ok {
				if !vAdded {
					groupVertexes.Add(vertex)
					vAdded = true
				}

				ml.lockSurface[path] = []lockRef{{t: refType, v: vertex}}

				atomic.AddInt64(&ml.statistics.lockrefCount, 1)

				continue
			}

			refInGroup := false
			refIsHead := false
			refIsWrite := false

			replaceAll := isHead && vertex.LockType() == LockTypeWrite
			preventAppend := false

			var ref lockRef
			// vertical iteration over existing lock refs on the path
			for i := len(existingRefs) - 1; i >= 0; i-- {
				ref = existingRefs[i]

				refIsHead = ref.t == head
				refInGroup = groupVertexes.Has(ref.v)
				refIsWrite = ref.v.LockType() == LockTypeWrite
				replaceCurrent := false

				if refInGroup && refIsHead && ref.v.LockType() >= lockType {
					break pathIteration
				}

				if !refInGroup && (refIsHead || isHead) {
					ref.v.AddChild(vertex)

					if !vAdded {
						groupVertexes.Add(vertex)
						vAdded = true
					}
				}

				if replaceAll && !refInGroup {
					replaceAll = false
				}

				if refInGroup {
					switch true {
					case refIsHead && refIsWrite:
						preventAppend = true
					case refIsHead && !refIsWrite:
						if isHead && lockType == LockTypeWrite {
							replaceCurrent = true
						}
					case !refIsHead && refIsWrite:
						if isHead && lockType == LockTypeWrite {
							replaceCurrent = true
						}
						if !isHead {
							preventAppend = true
						}
					case !refIsHead && !refIsWrite:
						if lockType == LockTypeWrite {
							replaceCurrent = true
						} else if isHead {
							replaceCurrent = true
						}
					}
				}

				if replaceCurrent {
					existingRefs[i] = lockRef{t: refType, v: vertex}
					preventAppend = true

					if !vAdded {
						groupVertexes.Add(vertex)
						vAdded = true
					}
				}

			}

			if replaceAll {
				ml.lockSurface[path] = []lockRef{{t: refType, v: vertex}}
				atomic.AddInt64(&ml.statistics.lockrefCount, int64(1-len(existingRefs)))
				if !vAdded {
					groupVertexes.Add(vertex)
					vAdded = true
				}
				break
			}

			if preventAppend {
				continue
			}

			atomic.AddInt64(&ml.statistics.lockrefCount, 1)
			ml.lockSurface[path] = append(existingRefs, lockRef{t: refType, v: vertex})
			if !vAdded {
				groupVertexes.Add(vertex)
				vAdded = true
			}
		}
	}

	ml.mx.Unlock()

	vertexes := groupVertexes.GetAll()

	go ml.handleUnlocker(u, vertexes, lockGroup, tokenRefGroup)

	l := Lock{
		u:  u,
		ch: make(chan struct{}, 1),
		id: lockID,
	}

	atomic.AddInt64(&ml.statistics.pendingVertexCount, int64(len(vertexes)))

	for i, v := range vertexes {
		vertexLock := v.LockChan()

		select {
		// Try lock immediately if possible to return an acquired lock
		case <-vertexLock:
			atomic.AddInt64(&ml.statistics.pendingVertexCount, -1)
			atomic.AddInt64(&ml.statistics.acquiredVertexCount, 1)
			continue
		// Otherwise, return non-acquired lock and do the locking in the background
		default:
			go func() {
				<-vertexLock
				atomic.AddInt64(&ml.statistics.pendingVertexCount, -1)
				atomic.AddInt64(&ml.statistics.acquiredVertexCount, 1)

				for _, v := range vertexes[i+1:] {
					v.Lock()
					atomic.AddInt64(&ml.statistics.pendingVertexCount, -1)
					atomic.AddInt64(&ml.statistics.acquiredVertexCount, 1)
				}

				atomic.AddInt64(&ml.statistics.groupsPending, -1)
				atomic.AddInt64(&ml.statistics.groupsAcquired, 1)

				l.makeReady(u)
			}()

			return l
		}
	}

	l.makeReady(u)

	atomic.AddInt64(&ml.statistics.groupsPending, -1)
	atomic.AddInt64(&ml.statistics.groupsAcquired, 1)

	return l
}

func (ml *MultiLocker) tokenizeSegments(segments []string) []token {
	refs := make([]token, len(segments))

	for i, s := range segments {
		p := ml.segmentTokens.Store(s)

		refs[i] = token(unsafe.Pointer(p))
	}

	return refs
}

func (ml *MultiLocker) releaseSegments(segments []string) {
	for _, s := range segments {
		ml.segmentTokens.Release(s)
	}
}

func (ml *MultiLocker) cleanPaths() {
	opened := true

	for opened {
		tokenGroupList := make([][][]token, 0, 1)
		var segmentGroup [][]token

		segmentGroup, opened = <-ml.garbage
		if !opened {
			break
		}

		tokenGroupList = append(tokenGroupList, segmentGroup)

		garbageRemains := true
		for garbageRemains {
			select {
			case segmentGroup, opened = <-ml.garbage:
				if !opened {
					garbageRemains = false
					break
				}
				tokenGroupList = append(tokenGroupList, segmentGroup)
			default:
				garbageRemains = false
			}
		}

		paths := make(set.Set[string])

		buffer := newTokenBuffer(tokenBufferInitialSize)

		for _, tokenGroup := range tokenGroupList {
			for _, tokenRefs := range tokenGroup {
				if len(tokenRefs) > len(buffer) {
					buffer = newTokenBuffer(len(tokenRefs) * 2)
				}

				for i := range tokenRefs {
					paths.Add(concatTokenRefs(tokenRefs[:i+1], buffer))
				}
			}
		}

		ml.mx.Lock()

		for path := range paths {
			refStack, ok := ml.lockSurface[path]
			if !ok {
				continue
			}

			keepFrom := 0

			for i, tokenRef := range refStack {
				if !tokenRef.v.Useless() {
					break
				}

				keepFrom = i + 1
			}

			if keepFrom == 0 {
				continue
			}

			atomic.AddInt64(&ml.statistics.lockrefCount, -int64(keepFrom))

			if keepFrom == len(refStack) {
				delete(ml.lockSurface, path)
				continue
			}

			ml.lockSurface[path] = refStack[keepFrom:]
		}

		ml.mx.Unlock()
	}

	ml.cleaned <- struct{}{}
}

func (ml *MultiLocker) handleUnlocker(u Unlocker, vertexes []*dagLock.Vertex, resourceLocks []ResourceLock, tokenRefGroup [][]token) {
	unlockCallback := <-u.ch

	ml.mx.Lock()

	vertexesInUse := make([]*dagLock.Vertex, 0)

	for _, v := range vertexes {
		v.Unlock()

		if !v.Useless() {
			vertexesInUse = append(vertexesInUse, v)
		}
	}

	atomic.AddInt64(&ml.statistics.acquiredVertexCount, -int64(len(vertexes)))
	atomic.AddInt64(&ml.statistics.groupsAcquired, -1)

	close(unlockCallback)

	for _, l := range resourceLocks {
		ml.releaseSegments(l.Path)
	}

	vw := dagLock.NewVertex(LockTypeWrite)

	// Read Vertexes may still have parents. If so, we need to ensure they are unlocked before trying to clean the paths.
	if len(vertexesInUse) > 0 {

		for _, v := range vertexesInUse {
			v.AddChild(vw)
		}
	}

	ml.mx.Unlock()

	vw.Lock()
	_ = 0 // get rid of "empty critical section" warning message
	vw.Unlock()

	ml.garbage <- tokenRefGroup

	ml.activeLockers.Done()
}
