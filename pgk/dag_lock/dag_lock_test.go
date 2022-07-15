package internal

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewVertice_ShoudBeInStateCreated(t *testing.T) {
	v := NewVertice(LockTypeWrite, "v1")

	if v.lockState != Created {
		t.Error("Newly created vertice is not in state Ready")
	}
}

func TestAddChild_AddNilChild(t *testing.T) {
	v := NewVertice(LockTypeWrite, "v1")

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v.AddChild(nil)
}

func TestAddChild_SelfAppend(t *testing.T) {
	v := NewVertice(LockTypeWrite, "v1")

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v.AddChild(v)
}

func TestAddChild_AddWithChildren(t *testing.T) {
	v1 := NewVertice(LockTypeWrite, "v1")
	v1.AddChild(NewVertice(LockTypeRead, ""))

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	NewVertice(LockTypeWrite, "v1").AddChild(v1)
}

func TestNewVertice_InitialLock(t *testing.T) {
	v1 := NewVertice(LockTypeWrite, "v1")

	v1.Lock()
}

func TestNewVertice_DoubleLock(t *testing.T) {
	v1 := NewVertice(LockTypeWrite, "v1")

	v1.Lock()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v1.Lock()
}

func TestNewVertice_LockUnlock(t *testing.T) {
	v1 := NewVertice(LockTypeWrite, "v1")

	v1.Lock()

	_ = 0

	v1.Unlock()
}

func TestNewVertice_UnlockBeforeLock(t *testing.T) {
	v1 := NewVertice(LockTypeWrite, "v1")

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic, got nil")
		}
	}()

	v1.Unlock()
}

func TestLockChan_ImmediateReceive(t *testing.T) {
	v := NewVertice(LockTypeWrite, "v1")

	select {
	case <-v.LockChan():
		return
	default:
		t.Error("Expected to receive from LockChan immediately")
	}
}

func TestLockChan_NoReceiveLocked(t *testing.T) {
	v1 := NewVertice(LockTypeWrite, "v1")

	v2 := NewVertice(LockTypeWrite, "v2")
	v1.AddChild(v2)

	select {
	case <-v2.LockChan():
		t.Error("Expected not to receive from LockChan")
	default:
		return
	}
}

func TestLockChan_WaitIfLocked(t *testing.T) {
	v1 := NewVertice(LockTypeWrite, "v1")

	v2 := NewVertice(LockTypeWrite, "v2")
	v1.AddChild(v2)

	go func() {
		v1.Lock()
		v1.Unlock()
	}()

	<-v2.LockChan()
}

func TestNewVertice_SequentialOrder_LockBeforeAddChild(t *testing.T) {
	order := make([]int, 0)

	v1 := NewVertice(LockTypeWrite, "v1")
	v2 := NewVertice(LockTypeWrite, "v2")
	v3 := NewVertice(LockTypeRead, "v3")
	v4 := NewVertice(LockTypeWrite, "v4")

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

	v1 := NewVertice(LockTypeWrite, "v1")
	v2 := NewVertice(LockTypeWrite, "v2")
	v3 := NewVertice(LockTypeRead, "v3")
	v4 := NewVertice(LockTypeWrite, "v4")

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

	v1 := NewVertice(LockTypeWrite, "v1")
	v2 := NewVertice(LockTypeRead, "v2")
	v3 := NewVertice(LockTypeRead, "v3")
	v4 := NewVertice(LockTypeRead, "v4")
	v5 := NewVertice(LockTypeWrite, "v5")

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

	v1 := NewVertice(LockTypeWrite, "v1")
	v2 := NewVertice(LockTypeRead, "v2")
	v3 := NewVertice(LockTypeRead, "v3")
	v4 := NewVertice(LockTypeRead, "v4")

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

		v5 := NewVertice(LockTypeRead, "v5")
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

	v1 := NewVertice(LockTypeWrite, "v1")
	v2 := NewVertice(LockTypeRead, "v2")
	v3 := NewVertice(LockTypeRead, "v3")
	v4 := NewVertice(LockTypeRead, "v4")

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

		v5 := NewVertice(LockTypeWrite, "v5")
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
	v1 := NewVertice(LockTypeWrite, "v1")
	v2 := NewVertice(LockTypeRead, "v2")
	v3 := NewVertice(LockTypeRead, "v3")

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
	v1 := NewVertice(LockTypeWrite, "v1")
	v2 := NewVertice(LockTypeRead, "v2")
	v3 := NewVertice(LockTypeRead, "v3")

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
