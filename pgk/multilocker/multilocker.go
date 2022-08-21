package multilocker

import (
	"sync"
	"sync/atomic"
	"time"
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

// LockSpace allows you to acquire an atomic lock for a set of ResourceLocks.
// Use NewLockSpaceRun() to create a new LockSpace and Close() to finish the goroutines it spawns.
type LockSpace struct {
	mx              sync.Mutex
	segmentTokens   setCounter.SetCounter
	lockSurface     map[string][]lockRef
	garbage         chan [][]tokenRef
	rootRef         tokenRef
	activeLockers   *sync.WaitGroup
	cleanerFinished chan struct{}
	closed          int32
	statistics      lockSpaceStatistics
	lastLockID      int64
}

// Statistics represents current state of LockSpace. All values (except LastGroupID) are non-accumulative
type Statistics struct {
	LastGroupID    int64 // Sequence number of the last group (starting from 1)
	GroupsPending  int64 // number of groups waiting for acquiring locks for all their resources
	GroupsAcquired int64 // number of groups waiting acquired locks for all their resources
	LocksPending   int64 // number of resource locks being stored
	LocksAcquired  int64 // number of resource locks being stored
	LockrefCount   int64 // number of references to vertexes being stored in refStacks
	TokensTotal    int64 // number of unique tokens (parts of a path) being stored
	TokensUnique   int64 // number of unique tokens (parts of a path) being stored
	PathCount      int64 // number of unique paths requested. There is a refStack with lockRefs for each path. Initially, LockSpace has PathCount = 1 (for the root segment)
}

type lockSpaceStatistics struct {
	groupsPending       int64
	groupsAcquired      int64
	pendingVertexCount  int64
	acquiredVertexCount int64
	lockrefCount        int64
}

func NewLockSpace() *LockSpace {
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

// Close forbids making new locks and returns when the cleaner finishes with the remaining garbage.
func (ls *LockSpace) Close() {
	if atomic.AddInt32(&ls.closed, 1) > 1 {
		panic("LockSpace is already closed")
	}

	ls.activeLockers.Wait()

	close(ls.garbage)

	<-ls.cleanerFinished
}

func (ls *LockSpace) Statistics() Statistics {
	ls.mx.Lock()
	defer ls.mx.Unlock()

	s := Statistics{}

	s.LastGroupID = ls.lastLockID

	s.GroupsPending = atomic.LoadInt64(&ls.statistics.groupsPending)
	s.GroupsAcquired = atomic.LoadInt64(&ls.statistics.groupsAcquired)

	s.LocksPending = atomic.LoadInt64(&ls.statistics.pendingVertexCount)
	s.LocksAcquired = atomic.LoadInt64(&ls.statistics.acquiredVertexCount)

	s.LockrefCount = atomic.LoadInt64(&ls.statistics.lockrefCount)

	s.PathCount = int64(len(ls.lockSurface))
	s.TokensTotal = int64(ls.segmentTokens.Sum())
	s.TokensUnique = int64(ls.segmentTokens.Count())

	return s
}

// Lock is used to lock a group of resourceLock's.
// You may pass you own unlocker as the second argument (unlock) and use it to unlock the group.
// The returned value can be used to receive the reference to the second argument (unlock) if provided.
// If unlock is not provided, it is made internally. This is the preferred way to ensure you won't unlock the group before it acquires the lock.
func (ls *LockSpace) Lock(resourceLocks []ResourceLock, unlocker ...Unlocker) Lock {
	ls.activeLockers.Add(1)

	if atomic.LoadInt32(&ls.closed) > 0 {
		panic("LockSpace is closed")
	}

	var u Unlocker

	if len(unlocker) > 1 {
		panic("Passed more than one unlocker. Review your logic")
	} else if len(unlocker) == 1 {
		u = unlocker[0]
	} else {
		u = NewUnlocker()
	}

	locker := ls.lockResources(resourceLocks, u)

	return locker
}

func (ls *LockSpace) lockResources(lockGroup []ResourceLock, u Unlocker) Lock {
	ls.mx.Lock()

	ls.lastLockID++
	lockID := ls.lastLockID

	atomic.AddInt64(&ls.statistics.groupsPending, 1)

	tokenRefGroup := make([][]tokenRef, len(lockGroup))

	for i, record := range lockGroup {
		tokenRefGroup[i] = append([]tokenRef{ls.rootRef}, ls.storeTokens(record.Path)...)
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

	lockPathIteration: // horizontal iteration over path segments
		for i := range tokenRefs {
			path := concatTokenRefs(tokenRefs[:i+1], buffer)

			refType := tail
			isHead := false
			if i == len(tokenRefs)-1 {
				refType = head
				isHead = true
			}

			existingRefs, ok := ls.lockSurface[path]
			if !ok {
				if !vAdded {
					groupVertexes.Add(vertex)
					vAdded = true
				}

				ls.lockSurface[path] = []lockRef{{t: refType, v: vertex}}

				atomic.AddInt64(&ls.statistics.lockrefCount, 1)

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
					break lockPathIteration
				}

				// binding
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
				ls.lockSurface[path] = []lockRef{{t: refType, v: vertex}}
				atomic.AddInt64(&ls.statistics.lockrefCount, int64(1-len(existingRefs)))
				if !vAdded {
					groupVertexes.Add(vertex)
					vAdded = true
				}
				break
			}

			if preventAppend {
				continue
			}

			atomic.AddInt64(&ls.statistics.lockrefCount, 1)
			ls.lockSurface[path] = append(existingRefs, lockRef{t: refType, v: vertex})
			if !vAdded {
				groupVertexes.Add(vertex)
				vAdded = true
			}
		}
	}

	ls.mx.Unlock()

	vertexes := groupVertexes.GetAll()

	go ls.handleUnlocker(u, vertexes, lockGroup, tokenRefGroup)

	l := Lock{
		u:  u,
		ch: make(chan struct{}, 1),
		id: lockID,
	}

	atomic.AddInt64(&ls.statistics.pendingVertexCount, int64(len(vertexes)))

	for i, v := range vertexes {
		vertexLock := v.LockChan()

		select {
		case <-vertexLock:
			atomic.AddInt64(&ls.statistics.pendingVertexCount, -1)
			atomic.AddInt64(&ls.statistics.acquiredVertexCount, 1)
			continue
		default:
			go func() {
				<-vertexLock
				atomic.AddInt64(&ls.statistics.pendingVertexCount, -1)
				atomic.AddInt64(&ls.statistics.acquiredVertexCount, 1)

				for _, v := range vertexes[i+1:] {
					v.Lock()
					atomic.AddInt64(&ls.statistics.pendingVertexCount, -1)
					atomic.AddInt64(&ls.statistics.acquiredVertexCount, 1)
				}

				atomic.AddInt64(&ls.statistics.groupsPending, -1)
				atomic.AddInt64(&ls.statistics.groupsAcquired, 1)

				l.makeReady(u)
			}()

			return l
		}
	}

	l.makeReady(u)

	atomic.AddInt64(&ls.statistics.groupsPending, -1)
	atomic.AddInt64(&ls.statistics.groupsAcquired, 1)

	return l
}

func (ls *LockSpace) storeTokens(tokens []string) []tokenRef {
	refs := make([]tokenRef, len(tokens))

	for i, s := range tokens {
		p := ls.segmentTokens.Store(s)

		refs[i] = tokenRef(unsafe.Pointer(p))
	}

	return refs
}

func (ls *LockSpace) releaseTokens(tokens []string) {
	for _, s := range tokens {
		ls.segmentTokens.Release(s)
	}
}

func (ls *LockSpace) cleanRefStacks() {
	opened := true

	for opened {
		tokenGroupList := make([][][]tokenRef, 0, 1)
		var segmentGroup [][]tokenRef

		segmentGroup, opened = <-ls.garbage
		if !opened {
			break
		}

		tokenGroupList = append(tokenGroupList, segmentGroup)

	remainingGarbage:
		for {
			select {
			case segmentGroup, opened = <-ls.garbage:
				if !opened {
					break remainingGarbage
				}
				tokenGroupList = append(tokenGroupList, segmentGroup)
			default:
				break remainingGarbage
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

		ls.mx.Lock()

		for path := range paths {
			refStack, ok := ls.lockSurface[path]
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

			atomic.AddInt64(&ls.statistics.lockrefCount, -int64(keepFrom))

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

func (ls *LockSpace) handleUnlocker(u Unlocker, vertexes []*dagLock.Vertex, resourceLocks []ResourceLock, tokenRefGroup [][]tokenRef) {
	unlockProcessed := <-u.ch

	ls.mx.Lock()

	vertexesInUse := make([]*dagLock.Vertex, 0)

	for _, v := range vertexes {
		v.Unlock()

		if !v.Useless() {
			vertexesInUse = append(vertexesInUse, v)
		}
	}

	atomic.AddInt64(&ls.statistics.acquiredVertexCount, -int64(len(vertexes)))
	atomic.AddInt64(&ls.statistics.groupsAcquired, -1)

	close(unlockProcessed)

	for _, l := range resourceLocks {
		ls.releaseTokens(l.Path)
	}

	vw := dagLock.NewVertex(LockTypeWrite)

	// Read Vertexes may still have parents. If so, we need to ensure they are unlocked before trying to clean the paths.
	if len(vertexesInUse) > 0 {

		for _, v := range vertexesInUse {
			time.Sleep(time.Millisecond * 1)
			v.AddChild(vw)
		}
	}

	ls.mx.Unlock()

	vw.Lock()
	_ = 0 // get rid of "empty critical section" warning message
	vw.Unlock()

	ls.garbage <- tokenRefGroup

	ls.activeLockers.Done()
}
