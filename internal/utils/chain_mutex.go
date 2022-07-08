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
	selfMx           *sync.Mutex
	childCM          *ChainMutex
	_mx              sync.RWMutex
	lockType         LockType
	hasBeenLocked    bool
	hasBeenUnlocked  bool
	isParentUnlocked bool
}

func NewChainMutex(lockType LockType) *ChainMutex {
	cm := &ChainMutex{
		selfMx:           &sync.Mutex{},
		lockType:         lockType,
		isParentUnlocked: true,
	}

	return cm
}

func (cm *ChainMutex) Lock() {

	cm._mx.Lock()

	if cm.hasBeenLocked {
		panic("Unable to lock: ChainMutex has already been locked. Fix your logic")
	}

	cm.hasBeenLocked = true

	cm._mx.Unlock()

	cm.selfMx.Lock()
}

func (cm *ChainMutex) Unlock() {

	cm._mx.Lock()
	defer cm._mx.Unlock()

	if !cm.hasBeenLocked {
		panic("Unable to unlock: Do not call Unlock before calling Lock")
	}

	cm.hasBeenUnlocked = true

	cm.selfMx.Unlock()

	if cm.hasChild() {

		if cm.isParentUnlocked {
			cm.childCM.markParentUnlocked()
		}

		if cm.isChildConcurrent() {
			cm.releaseReadChain()
			return
		}

		if cm.isParentUnlocked {
			cm.childCM.selfMx.Unlock()
		}
	}
}

// Chain returns a new ChainMutex. It will not acquire you a lock until its parent is unlocked. Do not call Chain twice on the same ChainMutex.
func (cm *ChainMutex) Chain(lockType LockType) *ChainMutex {
	cm._mx.Lock()
	defer cm._mx.Unlock()

	if cm.childCM != nil {
		panic("child mutex already exists")
	}

	cm.childCM = NewChainMutex(lockType)
	cm.childCM.isParentUnlocked = cm.hasBeenUnlocked

	if !cm.hasBeenUnlocked && !cm.isChildConcurrent() {
		cm.childCM.selfMx.Lock()
	}

	return cm.childCM
}

func (cm *ChainMutex) hasChild() bool {
	return cm.childCM != nil
}

func (cm *ChainMutex) markParentUnlocked() {
	cm._mx.Lock()
	defer cm._mx.Unlock()

	cm.isParentUnlocked = true
}

func (cm *ChainMutex) isChildConcurrent() bool {
	if cm.lockType == LockTypeRead && cm.childCM.lockType == LockTypeRead {
		return true
	}

	return false
}

func (cm *ChainMutex) releaseReadChain() {

	if !cm.isParentUnlocked {
		return
	}

	if !cm.hasBeenUnlocked {
		return
	}

	if !cm.hasChild() {
		return
	}

	if cm.isChildConcurrent() {
		cm.childCM.markParentUnlocked()

		cm.childCM.releaseReadChain()

		return
	}

	cm.childCM.selfMx.Unlock()
}
