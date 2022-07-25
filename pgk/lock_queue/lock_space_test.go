package internal

import (
	"reflect"
	"sync"
	"testing"

	internal "github.com/xshkut/distributed-lock/pgk/dag_lock"
)

func assertWaiterIsWaiting(t *testing.T, lw Lock) {
	select {
	case <-lw.ch:
		t.Error("Waiter should still wait")
	default:
	}
}

func assertWaiterWontWait(t *testing.T, lw Lock) {
	select {
	case <-lw.ch:
	default:
		t.Error("Waiter should have completed")
	}
}

func assertOrder(t *testing.T, order []int, expected []int) {
	if !reflect.DeepEqual(order, expected) {
		t.Errorf("Order is %v, expected %v", order, expected)
	}
}

func TestLockSpace_SecondArgumentIsReceivedFromChan(t *testing.T) {
	ls := NewLockSpace()

	lr := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "c"})

	ul := NewUnlocker()
	unlock := ls.LockGroup([]resourceLock{lr}, ul).Acquire()

	if unlock != ul {
		t.Error("Second argument should be received from chan")
	}
}

func TestLockSpace_SingleGroupShouldBeLockedImmediately(t *testing.T) {
	ls := NewLockSpace()

	lr := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "c"})

	ls.LockGroup([]resourceLock{lr})
}

func TestLockSpace_DuplicateRecordsShouldNotBringDeadlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})
	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})

	ls.LockGroup([]resourceLock{lr1, lr2})
}

func TestLockSpace_ConcurrentGroupShouldBlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})
	ls.LockGroup([]resourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})

	w := ls.LockGroup([]resourceLock{lr2})
	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_EmptyPathShouldAlsoCauseBlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{})
	ls.LockGroup([]resourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})
	w := ls.LockGroup([]resourceLock{lr2})
	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_TestRelease(t *testing.T) {
	ls := NewLockSpace()

	path := []string{"a", "b", "c"}

	lr1 := NewResourceLock(internal.LockTypeWrite, path)
	w1 := ls.LockGroup([]resourceLock{lr1})
	w2 := ls.LockGroup([]resourceLock{lr1})

	assertWaiterWontWait(t, w1)
	assertWaiterIsWaiting(t, w2)

	unlocker := w1.Acquire()

	assertWaiterIsWaiting(t, w2)

	unlocker.Unlock()

	w2.Acquire()
}

func TestLockSpace_ParallelWritesShouldSucced(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "1"})
	w := ls.LockGroup([]resourceLock{lr1})
	assertWaiterWontWait(t, w)

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2"})
	w = ls.LockGroup([]resourceLock{lr2})
	assertWaiterWontWait(t, w)
}

func TestLockSpace_ParallelReadsShouldSucced(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1"})
	w := ls.LockGroup([]resourceLock{lr1})
	assertWaiterWontWait(t, w)

	lr2 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2"})
	w = ls.LockGroup([]resourceLock{lr2})
	assertWaiterWontWait(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Postfix(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b"})
	ls.LockGroup([]resourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2"})
	w := ls.LockGroup([]resourceLock{lr2})

	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Prefix(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2"})
	ls.LockGroup([]resourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b"})
	w := ls.LockGroup([]resourceLock{lr2})

	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_AdjacentReadsDoNotBlockEachOther(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a"})
	w1 := ls.LockGroup([]resourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeRead, []string{"a", "b"})
	w2 := ls.LockGroup([]resourceLock{lr2})

	lr3 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1"})
	w3 := ls.LockGroup([]resourceLock{lr3})

	lr4 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2"})
	w4 := ls.LockGroup([]resourceLock{lr4})

	lr5 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1", "a"})
	w5 := ls.LockGroup([]resourceLock{lr5})

	lr6 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1", "b"})
	w6 := ls.LockGroup([]resourceLock{lr6})

	lr7 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2", "a"})
	w7 := ls.LockGroup([]resourceLock{lr7})

	lr8 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2", "b"})
	w8 := ls.LockGroup([]resourceLock{lr8})

	lr9 := NewResourceLock(internal.LockTypeRead, []string{})
	w9 := ls.LockGroup([]resourceLock{lr9})

	lr10 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2", "b", "c"})
	w10 := ls.LockGroup([]resourceLock{lr10})

	assertWaiterWontWait(t, w1)

	assertWaiterIsWaiting(t, w2)
	assertWaiterIsWaiting(t, w3)
	assertWaiterIsWaiting(t, w4)
	assertWaiterIsWaiting(t, w5)
	assertWaiterIsWaiting(t, w6)
	assertWaiterIsWaiting(t, w7)
	assertWaiterIsWaiting(t, w8)
	assertWaiterIsWaiting(t, w9)

	assertWaiterIsWaiting(t, w10)

	u := w1.Acquire()

	assertWaiterIsWaiting(t, w2)
	assertWaiterIsWaiting(t, w3)
	assertWaiterIsWaiting(t, w4)
	assertWaiterIsWaiting(t, w5)
	assertWaiterIsWaiting(t, w6)
	assertWaiterIsWaiting(t, w7)
	assertWaiterIsWaiting(t, w8)
	assertWaiterIsWaiting(t, w9)

	assertWaiterIsWaiting(t, w10)

	u.Unlock()

	u9 := w9.Acquire()
	u8 := w8.Acquire()
	u7 := w7.Acquire()
	u6 := w6.Acquire()
	u5 := w5.Acquire()
	u4 := w4.Acquire()
	u3 := w3.Acquire()
	u2 := w2.Acquire()

	assertWaiterIsWaiting(t, w10)

	for _, u := range []Unlocker{u9, u8, u7, u6, u5, u4, u3, u2} {
		u.Unlock()
	}

	w10.Acquire()
}

func TestLockSpace_PartialWriteOverlapping(t *testing.T) {
	ls := NewLockSpace()

	rl1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	rl2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "2"})
	rl3 := NewResourceLock(internal.LockTypeWrite, []string{"a", "3"})
	w1 := ls.LockGroup([]resourceLock{rl1, rl2, rl3})

	rl4 := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	rl5 := NewResourceLock(internal.LockTypeRead, []string{"a", "4"})
	rl6 := NewResourceLock(internal.LockTypeRead, []string{"a", "5"})
	w2 := ls.LockGroup([]resourceLock{rl4, rl5, rl6})

	rl7 := NewResourceLock(internal.LockTypeWrite, []string{"a", "5"})
	rl8 := NewResourceLock(internal.LockTypeWrite, []string{"a", "6"})
	rl9 := NewResourceLock(internal.LockTypeWrite, []string{"a", "7"})
	w3 := ls.LockGroup([]resourceLock{rl7, rl8, rl9})

	assertWaiterIsWaiting(t, w2)
	assertWaiterIsWaiting(t, w3)

	w1.Acquire().Unlock()

	assertWaiterIsWaiting(t, w3)

	w2.Acquire().Unlock()

	w3.Acquire().Unlock()
}

func TestLockSpace_PartialReadOverlapping(t *testing.T) {
	ls := NewLockSpace()

	rl1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	rl2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "2"})
	rl3 := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	w1 := ls.LockGroup([]resourceLock{rl1, rl2, rl3})

	rl4 := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	rl5 := NewResourceLock(internal.LockTypeWrite, []string{"a", "4"})
	rl6 := NewResourceLock(internal.LockTypeRead, []string{"a", "5"})
	w2 := ls.LockGroup([]resourceLock{rl4, rl5, rl6})

	rl7 := NewResourceLock(internal.LockTypeRead, []string{"a", "5"})
	rl8 := NewResourceLock(internal.LockTypeWrite, []string{"a", "6"})
	rl9 := NewResourceLock(internal.LockTypeWrite, []string{"a", "7"})
	w3 := ls.LockGroup([]resourceLock{rl7, rl8, rl9})

	w3.Acquire()
	w2.Acquire()
	w1.Acquire()
}

func TestLockSpace_Complex_1(t *testing.T) {
	ls := NewLockSpace()
	order := []int{}

	lr1 := NewResourceLock(internal.LockTypeRead, []string{"a"})
	w1 := ls.LockGroup([]resourceLock{lr1})

	lr2a := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	lr2b := NewResourceLock(internal.LockTypeWrite, []string{"a", "2"})
	w2 := ls.LockGroup([]resourceLock{lr2a, lr2b})

	lr3a := NewResourceLock(internal.LockTypeRead, []string{})
	w3a := ls.LockGroup([]resourceLock{lr3a})

	lr3b := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	w3b := ls.LockGroup([]resourceLock{lr3b})

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		u := w3b.Acquire()
		order = append(order, 3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w3a.Acquire()
		order = append(order, 3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w2.Acquire()
		order = append(order, 2)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w1.Acquire()
		order = append(order, 1)
		u.Unlock()

		wg.Done()
	}()

	wg.Wait()

	assertOrder(t, order, []int{1, 2, 3})
}

func TestLockSpace_Complex_2(t *testing.T) {
	ls := NewLockSpace()
	order := []int{}

	wg := sync.WaitGroup{}
	wg.Add(10)

	// 1
	r1 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "c", "d"})
	w1 := ls.LockGroup([]resourceLock{r1})

	// 2
	r2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b"})
	w2 := ls.LockGroup([]resourceLock{r2})

	// 3 ...
	r3 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "1", "a"})
	w3 := ls.LockGroup([]resourceLock{r3})

	r4 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "1", "b"})
	w4 := ls.LockGroup([]resourceLock{r4})

	r5 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2", "a"})
	w5 := ls.LockGroup([]resourceLock{r5})

	r6 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2", "b"})
	w6 := ls.LockGroup([]resourceLock{r6})

	// 4 ...
	r7 := NewResourceLock(internal.LockTypeRead, []string{})
	w7 := ls.LockGroup([]resourceLock{r7})

	r8 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "3"})
	w8 := ls.LockGroup([]resourceLock{r8})

	r9 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "3", "a"})
	w9 := ls.LockGroup([]resourceLock{r9})

	r10 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "3", "a", "b"})
	w10 := ls.LockGroup([]resourceLock{r10})

	go func() {
		u := w10.Acquire()
		order = append(order, 4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w9.Acquire()
		order = append(order, 4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w8.Acquire()
		order = append(order, 4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w7.Acquire()
		order = append(order, 4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w6.Acquire()
		order = append(order, 3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w5.Acquire()
		order = append(order, 3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w4.Acquire()
		order = append(order, 3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w3.Acquire()
		order = append(order, 3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w2.Acquire()
		order = append(order, 2)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w1.Acquire()
		order = append(order, 1)
		u.Unlock()

		wg.Done()
	}()

	wg.Wait()

	assertOrder(t, order, []int{1, 2, 3, 3, 3, 3, 4, 4, 4, 4})
}
