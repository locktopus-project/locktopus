package daglock

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewVertice_ShoudBeInStateCreated(t *testing.T) {
	v := NewVertex(LockTypeWrite)

	if v.lockState != Created {
		t.Error("Newly created vertice is not in state Ready")
	}
}

func TestAddChild_AddNilChild(t *testing.T) {
	v := NewVertex(LockTypeWrite)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v.AddChild(nil)
}

func TestAddChild_SelfAppend(t *testing.T) {
	v := NewVertex(LockTypeWrite)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v.AddChild(v)
}

func TestAddChild_AddWithChildren(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)
	v1.AddChild(NewVertex(LockTypeRead))

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	NewVertex(LockTypeWrite).AddChild(v1)
}

func TestNewVertice_InitialLock(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)

	v1.Lock()
}

func TestNewVertice_DoubleLock(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)

	v1.Lock()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v1.Lock()
}

func TestNewVertice_LockUnlock(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)

	v1.Lock()

	_ = 0

	v1.Unlock()
}

func TestNewVertice_UnlockBeforeLock(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v1.Unlock()
}

func TestLockChan_ImmediateReceive(t *testing.T) {
	v := NewVertex(LockTypeWrite)

	select {
	case <-v.LockChan():
		return
	default:
		t.Error("Expected to receive from LockChan immediately")
	}
}

func TestLockChan_NoReceiveLocked(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)

	v2 := NewVertex(LockTypeWrite)
	v1.AddChild(v2)

	select {
	case <-v2.LockChan():
		t.Error("Expected not to receive from LockChan")
	default:
		return
	}
}

func TestLockChan_ImmediateReceiveReadStacks(t *testing.T) {
	v1 := NewVertex(LockTypeRead)
	v2 := NewVertex(LockTypeRead)

	v1.AddChild(v2)

	select {
	case <-v2.LockChan():
		return
	default:
		t.Error("Expected to receive from LockChan immediately")
	}
}

func TestLockChan_WaitIfLocked(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)

	v2 := NewVertex(LockTypeWrite)
	v1.AddChild(v2)

	go func() {
		v1.Lock()
		_ = v1
		v1.Unlock()
	}()

	<-v2.LockChan()
}

func TestNewVertice_SequentialOrder_LockBeforeAddChild(t *testing.T) {
	order := make([]int, 0)

	v1 := NewVertex(LockTypeWrite)
	v2 := NewVertex(LockTypeWrite)
	v3 := NewVertex(LockTypeRead)
	v4 := NewVertex(LockTypeWrite)

	v1.Lock()
	order = append(order, 1)
	v1.AddChild(v2)
	v1.Unlock()

	v2.Lock()
	v2.AddChild(v3)
	order = append(order, 2)
	v2.Unlock()

	v3.Lock()
	v3.AddChild(v4)
	order = append(order, 3)
	v3.Unlock()

	v4.Lock()
	order = append(order, 4)
	v4.Unlock()

	if fmt.Sprintf("%v", order) != "[1 2 3 4]" {
		t.Errorf("Expected order to be [1 2 3 4], got %v", order)
	}
}

func TestNewVertice_SequentialOrder_AddChildBeforeLock(t *testing.T) {
	order := make([]int, 0)
	wg := sync.WaitGroup{}
	wg.Add(4)

	v1 := NewVertex(LockTypeWrite)
	v2 := NewVertex(LockTypeWrite)
	v3 := NewVertex(LockTypeRead)
	v4 := NewVertex(LockTypeWrite)

	v1.AddChild(v2)
	v2.AddChild(v3)
	v3.AddChild(v4)

	go func() {
		v4.Lock()
		order = append(order, 4)
		v4.Unlock()

		wg.Done()
	}()

	go func() {
		v3.Lock()
		order = append(order, 3)
		v3.Unlock()

		wg.Done()
	}()

	go func() {
		v2.Lock()
		order = append(order, 2)
		v2.Unlock()

		wg.Done()
	}()

	go func() {
		v1.Lock()
		order = append(order, 1)
		v1.Unlock()

		wg.Done()
	}()

	wg.Wait()

	if fmt.Sprintf("%v", order) != "[1 2 3 4]" {
		t.Errorf("Expected order to be [1 2 3 4], got %v", order)
	}
}

func TestNewVertice_StackedReadsMustBeLockedIndependently(t *testing.T) {
	order := make([]int, 0)
	wg := sync.WaitGroup{}
	wg.Add(3)

	v1 := NewVertex(LockTypeWrite)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeRead)
	v4 := NewVertex(LockTypeRead)
	v5 := NewVertex(LockTypeWrite)

	v1.AddChild(v2)
	v2.AddChild(v3)
	v3.AddChild(v4)
	v4.AddChild(v5)

	go func() {
		v5.Lock()
		order = append(order, 5)
		v5.Unlock()

		wg.Done()
	}()

	go func() {
		v3.Lock()
		order = append(order, 3)
		v3.Unlock()

		v4.Lock()
		order = append(order, 4)
		v4.Unlock()

		v2.Lock()
		order = append(order, 2)
		v2.Unlock()

		wg.Done()
	}()

	go func() {
		v1.Lock()
		order = append(order, 1)
		v1.Unlock()

		wg.Done()
	}()

	wg.Wait()

	if fmt.Sprintf("%v", order) != "[1 3 4 2 5]" {
		t.Errorf("Expected order to be [1 3 4 2 5], got %v", order)
	}
}

func TestNewVertice_EnsureTailingReadWorks(t *testing.T) {
	order := make([]int, 0)
	wg := sync.WaitGroup{}
	wg.Add(2)

	v1 := NewVertex(LockTypeWrite)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeRead)
	v4 := NewVertex(LockTypeRead)

	v1.AddChild(v2)
	v2.AddChild(v3)
	v3.AddChild(v4)

	go func() {
		v3.Lock()
		order = append(order, 3)
		v3.Unlock()

		v4.Lock()
		order = append(order, 4)
		v4.Unlock()

		v2.Lock()
		order = append(order, 2)
		v2.Unlock()

		v5 := NewVertex(LockTypeRead)
		v4.AddChild(v5)
		v5.Lock()
		order = append(order, 5)
		v5.Unlock()

		wg.Done()
	}()

	go func() {
		v1.Lock()
		order = append(order, 1)
		v1.Unlock()

		wg.Done()
	}()

	wg.Wait()

	if fmt.Sprintf("%v", order) != "[1 3 4 2 5]" {
		t.Errorf("Expected order to be [1 3 4 2 5], got %v", order)
	}
}

func TestNewVertice_EnsureTailingWriteWorks(t *testing.T) {
	order := make([]int, 0)
	wg := sync.WaitGroup{}
	wg.Add(2)

	v1 := NewVertex(LockTypeWrite)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeRead)
	v4 := NewVertex(LockTypeRead)

	v1.AddChild(v2)
	v2.AddChild(v3)
	v3.AddChild(v4)

	go func() {
		v3.Lock()
		order = append(order, 3)
		v3.Unlock()

		v4.Lock()
		order = append(order, 4)
		v4.Unlock()

		v2.Lock()
		order = append(order, 2)
		v2.Unlock()

		v5 := NewVertex(LockTypeWrite)
		v4.AddChild(v5)
		v5.Lock()
		order = append(order, 5)
		v5.Unlock()

		wg.Done()
	}()

	go func() {
		v1.Lock()
		order = append(order, 1)
		v1.Unlock()

		wg.Done()
	}()

	wg.Wait()

	if fmt.Sprintf("%v", order) != "[1 3 4 2 5]" {
		t.Errorf("Expected order to be [1 3 4 2 5], got %v", order)
	}
}

func TestNewVertice_EnsureStackedReadsAreAllBlocked(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeRead)

	v1.Lock()
	v1.AddChild(v2)
	v2.AddChild(v3)

	select {
	case <-v3.LockChan():
		t.Error("Second read is expected to be locked")
	default:
	}

	select {
	case <-v2.LockChan():
		t.Error("Third read is expected to be locked")
	default:
	}
}

func TestNewVertice_EnsureStackedWritesAreAllBlocked(t *testing.T) {
	v1 := NewVertex(LockTypeWrite)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeRead)

	v1.Lock()
	v1.AddChild(v2)
	v2.AddChild(v3)

	select {
	case <-v2.LockChan():
		t.Error("Third read is expected to be locked")
	default:
	}

	select {
	case <-v3.LockChan():
		t.Error("Second read is expected to be locked")
	default:
	}
}

func TestNewVertice_StackedReadsLockIndependently_2(t *testing.T) {
	v1 := NewVertex(LockTypeRead)
	v2 := NewVertex(LockTypeRead)

	v1.AddChild(v2)

	v2.Lock()
}

func TestNewVertice_UnlockedShouldHaveNotChildren(t *testing.T) {
	v1 := NewVertex(LockTypeRead)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeWrite)
	v4 := NewVertex(LockTypeWrite)
	v5 := NewVertex(LockTypeRead)
	v6 := NewVertex(LockTypeRead)
	v7 := NewVertex(LockTypeWrite)
	v8 := NewVertex(LockTypeRead)
	v9 := NewVertex(LockTypeWrite)

	v1.AddChild(v2)
	v2.AddChild(v3)
	v3.AddChild(v4)
	v4.AddChild(v5)
	v5.AddChild(v6)
	v6.AddChild(v7)
	v7.AddChild(v8)
	v8.AddChild(v9)

	v1.Lock()
	v1.Unlock()
	v2.Lock()
	v2.Unlock()

	if v1.HasChildren() {
		t.Error("Expected v1 to have no children")
	}
	if v2.HasChildren() {
		t.Error("Expected v2 to have no children")
	}

	v3.Lock()
	v3.Unlock()

	if v3.HasChildren() {
		t.Error("Expected v3 to have no children")
	}

	v4.Lock()
	v4.Unlock()

	if v4.HasChildren() {
		t.Error("Expected v4 to have no children")
	}

	v5.Lock()
	v5.Unlock()

	if v5.HasChildren() {
		t.Error("Expected v5 to have no children")
	}

	v6.Lock()
	v6.Unlock()

	if v6.HasChildren() {
		t.Error("Expected v6 to have no children")
	}

	v7.Lock()
	v7.Unlock()

	if v7.HasChildren() {
		t.Error("Expected v7 to have no children")
	}

	v8.Lock()
	v8.Unlock()

	if v8.HasChildren() {
		t.Error("Expected v8 to have no children")
	}
}

func TestAddChild_WriteAfterUnlockedStackedRead(t *testing.T) {
	v1 := NewVertex(LockTypeRead)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeWrite)

	v1.AddChild(v2)

	v2.Lock()
	v2.Unlock()

	v2.AddChild(v3)

	v3lock := v3.LockChan()

	select {
	case <-v3lock:
		t.Error("Expected v3 to be locked")
	default:
	}

	v1.Lock()
	v1.Unlock()

	<-v3lock
}

func TestUseless(t *testing.T) {
	v1 := NewVertex(LockTypeRead)
	v2 := NewVertex(LockTypeRead)
	v3 := NewVertex(LockTypeWrite)
	v4 := NewVertex(LockTypeWrite)

	if v1.Useless() {
		t.Error("Expected v1 not to be useless")
	}

	if v2.Useless() {
		t.Error("Expected v2 not to be useless")
	}

	if v3.Useless() {
		t.Error("Expected v3 not to be useless")
	}

	if v4.Useless() {
		t.Error("Expected v4 not to be useless")
	}

	v1.AddChild(v2)

	v2.Lock()
	v2.Unlock()

	if v2.Useless() {
		t.Error("Expected v1 not to be useless")
	}

	v2.AddChild(v3)
	v3.AddChild(v4)

	v1.Lock()
	v1.Unlock()

	if !v1.Useless() {
		t.Error("Expected v1 to be useless")
	}

	if !v2.Useless() {
		t.Error("Expected v1 to be useless")
	}

	v3.Lock()
	v3.Unlock()

	if !v3.Useless() {
		t.Error("Expected v3 to be useless")
	}

	v4.Lock()
	v4.Unlock()

	if !v4.Useless() {
		t.Error("Expected v4 to be useless")
	}
}
