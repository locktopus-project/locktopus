package internal

import (
	"sync"
	"unsafe"

	dagLock "github.com/xshkut/distributed-lock/pgk/dag_lock"
	"github.com/xshkut/distributed-lock/pgk/set"
	setCounter "github.com/xshkut/distributed-lock/pgk/set_counter"
)

type lease struct {
	lockType dagLock.LockType
	path     []string
}

func NewLease(lockType dagLock.LockType, path []string) lease {
	return lease{
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
}

type segmentRef uintptr

func NewLockSpace() *LockSpace {
	ls := LockSpace{
		segmentRegistry: setCounter.NewSetCounter(),
		lockSurface:     make(map[string][]lockRef),
	}

	ls.rootRef = ls.storeSegment("root")

	return &ls
}

// LockGroup returns chan which signals when all locks are acquired.
// Use second argument to unlock the group when the lock can be released.
// The returned chan can be used to retrieve the reference to the second argument. This way you can ensure not to unlock the group before it acquires the lock.
// It is safe to call LockGroup multiple times.
func (ls *LockSpace) LockGroup(leases []lease, unlock chan interface{}) <-chan chan<- interface{} {
	vertexes := make([]*dagLock.Vertex, len(leases))

	ls.mx.Lock()

	// paths := make([]string, len(leases))
	segmentRefses := make([][]segmentRef, len(leases))

	for i, record := range leases {
		segmentRefses[i] = append([]segmentRef{ls.rootRef}, ls.storeSegments(record.path)...)

		// paths[i] = concatSegmentRefs(segmentRefses[i])
	}

	groupVertexes := make(set.Set[*dagLock.Vertex])

	for i, segmentRefs := range segmentRefses {
		vertex := dagLock.NewVertex(leases[i].lockType)
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
		<-unlock

		ls.mx.Lock()
		defer ls.mx.Unlock()

		for _, v := range vertexes {
			v.Unlock()
		}

		for _, l := range leases {
			ls.releaseSegments(l.path)
		}

		// TODO: implement cleaning map
	}()

	ls.mx.Unlock()

	groupCh := make(chan chan<- interface{}, 1)

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

				groupCh <- unlock
				close(groupCh)
			}()

			return groupCh
		}

	}

	groupCh <- unlock
	close(groupCh)
	return groupCh
}

// func cleanLockSurface(ls *LockSpace, ch <-chan []segmentRef) {

// 	for refs := range ch {
// 		ls.mx.Lock()

// 		for i, _ := range refs {
// 			delete(ls.lockSurface, concatSegmentRefs(refs[:i+1]))
// 		}

// 		defer ls.mx.Unlock()
// 	}

// }

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
