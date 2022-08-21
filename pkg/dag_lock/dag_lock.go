/*
DAG-Lock implements a concept of unidirectional locks over directed acyclic graph (https://en.wikipedia.org/wiki/Directed_acyclic_graph).
Each vertex can be locked when all its parents have been locked-unlocked.
The exception is a sequence of read locks that can be acquired in parallel.
*/
package daglock

import (
	"fmt"
	"sync"

	internal "github.com/xshkut/gearlock/pkg/set"
)

// LockType can be LockTypeRead or LockTypeWrite (see sync.RWMutex).
type LockType int8

var lockTypeNames = []string{"read", "write"}

func (lt LockType) String() string {
	return lockTypeNames[lt]
}

const (
	LockTypeRead  LockType = iota
	LockTypeWrite LockType = iota
)

// LockState lifecycle of a Vertex: [Created] -> [LockedByParents] -> [LockedByClient] -> [Released] -> [Unlocked]
type LockState int8

const (
	Created         LockState = iota
	LockedByParents LockState = iota
	Released        LockState = iota
	LockedByClient  LockState = iota
	Unlocked        LockState = iota
)

// Vertex is a one-time-lock mutex, a part of the DAG-lock concept.
// Use NewVertex to create a new Vertex.
// All methods are thread-safe.
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

// AddChild sets c to be dependent on v.
// After v is unlocked, child will be ready to acquire the lock.
// Do not call Lock() or AddChild() on c before binding it to all necessary parents.
func (v *Vertex) AddChild(c *Vertex) {
	if c == nil {
		panic("Unable to append nil child. Fix your logic or report a bug")
	}
	if c == v {
		panic("Unable to append self. Fix your logic or report a bug")
	}

	v._mx.Lock()
	defer v._mx.Unlock()

	if c.HasChildren() {
		panic("Unable to bind a Vertex that already has children. This may introduce a deadlock. Fix your logic or report a bug")
	}

	if v.Useless() {
		return
	}

	c._mx.Lock()
	defer c._mx.Unlock()

	if c.lockState > LockedByParents {
		panic("Cannot bind released child. Fix your logic")
	}

	if !v.HasParents() && v.lockState < Released {
		v.lockState = Released
	}

	c.parents.Add(v)
	v.children.Add(c)

	if v.lockState > LockedByParents && c.lockType == LockTypeRead && v.lockType == LockTypeRead {
		return
	}

	if c.lockState == Created {
		c.selfMx.Lock()
		c.lockState = LockedByParents
	}
}

// Lock starts locking. It will not be acquired until all parents are unlocked.
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

// LockChan starts locking and returns chan that will emit a value when the lock is ready to be acquired. If the lock has been acquired immediately withing LockChan() call, the returned chan is ready for receving from.
// This method can be used with "select" statement to check whether the lock has been acquired immediately (see tests).
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

// Unlock unlocks the vertex. Call Unlock() only after calling Lock().
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

// Useless means that adding children to v has no point. However, doig so is not forbidden and will result in a no-op.
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
