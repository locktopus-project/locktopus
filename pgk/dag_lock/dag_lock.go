/*
https://en.wikipedia.org/wiki/Directed_acyclic_graph
*/
package internal

import (
	"fmt"
	"sync"
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

// Vertice is a linked-list-based one-time mutex. Use NewVertice to create a new Vertice.
type Vertice struct {
	_mx             sync.Mutex
	lockType        LockType
	lockState       LockState
	selfMx          sync.Mutex
	parents         []*Vertice
	children        []*Vertice
	releasedParents []*Vertice
	calledLock      bool
	name            string
}

// NewVerice
func NewVertice(lockType LockType, name string) *Vertice {
	v := &Vertice{
		lockType:        lockType,
		children:        make([]*Vertice, 0),
		parents:         make([]*Vertice, 0),
		releasedParents: make([]*Vertice, 0),

		name: name,
	}

	return v
}

func (v *Vertice) HasChildren() bool {
	return len(v.children) > 0
}

func (v *Vertice) AddChild(child *Vertice) {
	if child == nil {
		panic("Unable to append nil child. Fix your logic or report a bug")
	}
	if child == v {
		panic("Unable to append self. Fix your logic or report a bug")
	}

	v._mx.Lock()
	defer v._mx.Unlock()

	if child.HasChildren() {
		panic("Unable to bind a Vertice that already has children. This may introduce a deadlock. Fix your logic or report a bug")
	}

	if v.lockState == Unlocked {
		// no-op
		return
	}

	child._mx.Lock()
	defer child._mx.Unlock()

	if child.lockState > LockedByParents {
		panic("Cannot bind released child. Fix your logic")
	}

	child.parents = append(child.parents, v)
	v.children = append(v.children, child)

	if child.lockState == Created {
		child.selfMx.Lock()
		child.lockState = LockedByParents
	}
}

// Lock will not be acquired until all parents are unlocked.
func (v *Vertice) Lock() {
	v._mx.Lock()

	if v.calledLock {
		panic("Unable to lock: Vertice has already been locked. Fix your logic")
	}
	v.calledLock = true

	v._mx.Unlock()

	v.selfMx.Lock()

	v._mx.Lock()
	v.lockState = LockedByClient
	v._mx.Unlock()

	fmt.Println("v.lockState = LockedByClient", v.name)
}

// LockChain returns a channel which is immediately ready to receive from if the lock has been acquired.
// This is a helper method which may be used inside "select" statement.
func (v *Vertice) LockChan() <-chan interface{} {
	v._mx.Lock()

	if v.calledLock {
		panic("Unable to lock: Vertice has already been locked. Fix your logic")
	}
	v.calledLock = true

	v._mx.Unlock()
	ch := make(chan interface{}, 1)

	ctrlCh := make(chan interface{})

	go func() {
		v.selfMx.Lock()

		v._mx.Lock()
		v.lockState = LockedByClient
		v._mx.Unlock()

		ch <- struct{}{}
		close(ch)

		ctrlCh <- struct{}{}
	}()

	if v.lockState == LockedByParents {
		return ch
	}

	<-ctrlCh

	return ch
}

func (v *Vertice) Unlock() {
	v._mx.Lock()

	if v.lockState != LockedByClient {
		panic("Unable to unlock: Call Unlock only after acquiring Lock. lockState = " + fmt.Sprint(v.lockState))
	}

	v.selfMx.Unlock()
	v.lockState = Unlocked

	v._mx.Unlock()

	v.refreshState()
}

func (v *Vertice) allParentsReleased() bool {
	return len(v.releasedParents) == len(v.parents)
}

func (v *Vertice) hasParents() bool {
	return len(v.parents) > 0
}

func (v *Vertice) releaseReadParent(parent *Vertice) {
	v._mx.Lock()
	defer v._mx.Unlock()

	for _, releasedRead := range v.releasedParents {
		if releasedRead == parent {
			return
		}
	}

	v.releasedParents = append(v.releasedParents, parent)
}

func (v *Vertice) unbindParent(parent *Vertice) {
	v._mx.Lock()
	defer v._mx.Unlock()

	for i, releasedRead := range v.releasedParents {
		if releasedRead == parent {
			v.releasedParents = append(v.releasedParents[:i], v.releasedParents[i+1:]...)
			break
		}
	}

	for i, p := range v.parents {
		if p == parent {
			v.parents = append(v.parents[:i], v.parents[i+1:]...)
			break
		}
	}
}

func (v *Vertice) refreshState() {
	v._mx.Lock()
	defer v._mx.Unlock()

	if !v.hasParents() {
		if v.lockState == Unlocked {
			for _, node := range v.children {
				node.unbindParent(v)
				node.refreshState()
			}

			v.children = []*Vertice{}
			v.releasedParents = []*Vertice{}
		}
	}

	if v.allParentsReleased() {
		if v.lockState == LockedByParents {
			v.selfMx.Unlock()
			v.lockState = Released

			if v.lockType == LockTypeWrite {
				return
			}

			for _, node := range v.children {
				if node.lockType == LockTypeWrite {
					continue
				}

				node.releaseReadParent(v)
				node.refreshState()
			}
		}
	}
}
