package internal

import (
	"sync"
)

type LockType int

const (
	LockTypeRead  LockType = iota
	LockTypeWrite LockType = iota
)

// ChainMutex is a linked-list-based one-time mutex. Use NewChainMutex to create a new ChainMutex. Lock it only once
type ChainMutex struct {
	_internal_mx   sync.Mutex
	selfMx         sync.Mutex
	child          *ChainMutex
	lockType       LockType
	hasBeenLocked  bool
	unlocked       bool
	parentUnlocked bool
}

func NewChainMutex(lockType LockType) *ChainMutex {
	cm := &ChainMutex{
		lockType:       lockType,
		parentUnlocked: true,
	}

	return cm
}

func (cm *ChainMutex) Lock() {
	cm._internal_mx.Lock()

	if cm.hasBeenLocked {
		panic("Unable to lock: ChainMutex has already been locked. Fix your logic")
	}

	cm.hasBeenLocked = true

	cm._internal_mx.Unlock()

	cm.selfMx.Lock()
}

func (cm *ChainMutex) Unlock() {
	cm._internal_mx.Lock()
	defer cm._internal_mx.Unlock()

	if !cm.hasBeenLocked {
		panic("Unable to unlock: Do not call Unlock before calling Lock")
	}

	cm.unlocked = true

	cm.selfMx.Unlock()

	if !cm.hasChild() {
		return
	}

	if !cm.parentUnlocked {
		return
	}

	cm.child.markParentUnlocked()

	if cm.isChildConcurrent() {
		cm.releaseConcurrentChain()

		return
	}

	cm.child.selfMx.Unlock()
}

// Chain returns a new ChainMutex. It will not acquire you a lock until its parent is unlocked. Do not call Chain twice on the same ChainMutex.
func (cm *ChainMutex) Chain(lockType LockType) *ChainMutex {
	cm._internal_mx.Lock()
	defer cm._internal_mx.Unlock()

	if cm.child != nil {
		panic("child mutex already exists")
	}

	cm.child = NewChainMutex(lockType)
	cm.child.parentUnlocked = cm.unlocked

	if !cm.unlocked && !cm.isChildConcurrent() {
		cm.child.selfMx.Lock()
	}

	return cm.child
}

func (cm *ChainMutex) hasChild() bool {
	return cm.child != nil
}

func (cm *ChainMutex) markParentUnlocked() {
	cm._internal_mx.Lock()
	defer cm._internal_mx.Unlock()

	cm.parentUnlocked = true
}

func (cm *ChainMutex) isChildConcurrent() bool {
	if cm.lockType == LockTypeRead && cm.child.lockType == LockTypeRead {
		return true
	}

	return false
}

func (cm *ChainMutex) releaseConcurrentChain() {
	if !cm.parentUnlocked {
		return
	}

	if !cm.unlocked {
		return
	}

	if !cm.hasChild() {
		return
	}

	if cm.isChildConcurrent() {
		cm.child.markParentUnlocked()

		cm.child.releaseConcurrentChain()

		return
	}

	cm.child.selfMx.Unlock()
}
