/*
https://en.wikipedia.org/wiki/Directed_acyclic_graph
*/
package daglock

import (
	"fmt"
	"sync"

	internal "github.com/xshkut/distributed-lock/pgk/set"
)

type LockType int8

const (
	LockTypeRead  LockType = iota
	LockTypeWrite LockType = iota
)

// LockState lifecycle: LockedByParents -> Ready -> LockedByClient -> Unlocked
type LockState int8

const (
	Created         LockState = iota
	LockedByParents LockState = iota
	Released        LockState = iota
	LockedByClient  LockState = iota
	Unlocked        LockState = iota
)

// Vertex is a linked-list-based one-time mutex. Use NewVertex to create a new Vertex.
type Vertex struct {
	_mx             sync.Mutex
	lockType        LockType
	lockState       LockState
	selfMx          sync.Mutex
	parents         internal.Set[*Vertex]
	children        internal.Set[*Vertex]
	releasedParents internal.Set[*Vertex]
	calledLock      bool
}

// NewVerice
func NewVertex(lockType LockType) *Vertex {
	v := &Vertex{
		lockType:        lockType,
		children:        make(internal.Set[*Vertex], 0),
		parents:         make(internal.Set[*Vertex], 0),
		releasedParents: make(internal.Set[*Vertex], 0),
	}

	return v
}

func (v *Vertex) HasChildren() bool {
	return len(v.children) > 0
}

func (v *Vertex) AddChild(child *Vertex) {
	if child == nil {
		panic("Unable to append nil child. Fix your logic or report a bug")
	}
	if child == v {
		panic("Unable to append self. Fix your logic or report a bug")
	}

	v._mx.Lock()
	defer v._mx.Unlock()

	if child.HasChildren() {
		panic("Unable to bind a Vertex that already has children. This may introduce a deadlock. Fix your logic or report a bug")
	}

	if v.Useless() {
		return
	}

	child._mx.Lock()
	defer child._mx.Unlock()

	if child.lockState > LockedByParents {
		panic("Cannot bind released child. Fix your logic")
	}

	if !v.HasParents() && v.lockState < Released {
		v.lockState = Released
	}

	child.parents.Add(v)
	v.children.Add(child)

	if v.lockState > LockedByParents && child.lockType == LockTypeRead && v.lockType == LockTypeRead {
		return
	}

	if child.lockState == Created {
		child.selfMx.Lock()
		child.lockState = LockedByParents
	}
}

// Lock will not be acquired until all parents are unlocked.
func (v *Vertex) Lock() {
	v._mx.Lock()

	if v.calledLock {
		panic("Unable to lock: Vertex has already been locked. Fix your logic")
	}
	v.calledLock = true

	v._mx.Unlock()

	v.selfMx.Lock()

	v._mx.Lock()
	v.lockState = LockedByClient
	v._mx.Unlock()
}

// LockChain performs Lock and returns chan, waiting for result of which equals to waiting for Lock finish. If the lock has been acquired immediately, the returned chan is ready for receving in place.
// This is a helper method which may be used inside "select" statement.
func (v *Vertex) LockChan() <-chan struct{} {
	v._mx.Lock()
	defer v._mx.Unlock()

	if v.calledLock {
		panic("Unable to lock: Vertex has already been locked. Fix your logic")
	}
	v.calledLock = true

	ch := make(chan struct{})

	ok := v.selfMx.TryLock()
	if ok {
		v.lockState = LockedByClient

		close(ch)
	} else {
		go func() {
			v.selfMx.Lock()

			v._mx.Lock()
			defer v._mx.Unlock()

			v.lockState = LockedByClient

			close(ch)
		}()
	}

	return ch
}

func (v *Vertex) Unlock() {
	v._mx.Lock()

	if v.lockState != LockedByClient {
		panic("Unable to unlock: Call Unlock only after acquiring Lock. lockState = " + fmt.Sprint(v.lockState))
	}

	v.selfMx.Unlock()
	v.lockState = Unlocked

	v._mx.Unlock()

	v.refreshState()
}

// Useless means that adding children to v has no point. However, doig so is not forbidden and will result in no-op.
func (v *Vertex) Useless() bool {
	return v.lockState == Unlocked && !v.HasParents()
}

func (v *Vertex) LockType() LockType {
	return v.lockType
}

func (v *Vertex) LockState() LockState {
	return v.lockState
}

func (v *Vertex) allParentsReleased() bool {
	return len(v.releasedParents) == len(v.parents)
}

func (v *Vertex) HasParents() bool {
	return len(v.parents) > 0
}

func (v *Vertex) releaseReadParent(parent *Vertex) {
	v._mx.Lock()
	defer v._mx.Unlock()

	v.releasedParents.Add(parent)
}

func (v *Vertex) unbindParent(parent *Vertex) {
	v._mx.Lock()
	defer v._mx.Unlock()

	v.releasedParents.Remove(parent)
	v.parents.Remove(parent)
}

func (v *Vertex) refreshState() {
	v._mx.Lock()
	defer v._mx.Unlock()

	if !v.HasParents() {
		if v.lockState == Unlocked {
			for node := range v.children {
				node.unbindParent(v)
				node.refreshState()
			}

			v.children.Clear()
			v.releasedParents.Clear()
		}
	}

	if v.allParentsReleased() {
		if v.lockState == LockedByParents {
			v.selfMx.Unlock()
			v.lockState = Released

			if v.lockType == LockTypeWrite {
				return
			}

			for node := range v.children {
				if node.lockType == LockTypeWrite {
					continue
				}

				node.releaseReadParent(v)
				node.refreshState()
			}
		}
	}
}
