package lockqueue

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	internal "github.com/xshkut/distributed-lock/pgk/dag_lock"
	sliceAppender "github.com/xshkut/distributed-lock/pgk/slice_appender"
)

func assertWaiterIsWaiting(t *testing.T, lw GroupLocker) {
	select {
	case <-lw.ch:
		t.Error("Waiter should still wait")
	default:
	}
}

func assertWaiterWontWait(t *testing.T, lw GroupLocker) {
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
	ls := NewLockSpaceRun()

	lr := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "c"})

	ul := NewUnlocker()
	unlock := ls.LockGroup([]ResourceLock{lr}, ul).Acquire()

	if unlock != ul {
		t.Error("Second argument should be received from chan")
	}
}

func TestLockSpace_SingleGroupShouldBeLockedImmediately(t *testing.T) {
	ls := NewLockSpaceRun()

	lr := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "c"})

	ls.LockGroup([]ResourceLock{lr})
}

func TestLockSpace_DuplicateRecordsShouldNotBringDeadlock(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})
	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})

	ls.LockGroup([]ResourceLock{lr1, lr2})
}

func TestLockSpace_ConcurrentGroupShouldBlock(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})
	ls.LockGroup([]ResourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})

	w := ls.LockGroup([]ResourceLock{lr2})
	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_EmptyPathShouldAlsoCauseBlock(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{})
	ls.LockGroup([]ResourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "c"})
	w := ls.LockGroup([]ResourceLock{lr2})
	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_TestRelease(t *testing.T) {
	ls := NewLockSpaceRun()

	path := []string{"a", "b", "c"}

	lr1 := NewResourceLock(internal.LockTypeWrite, path)
	w1 := ls.LockGroup([]ResourceLock{lr1})
	w2 := ls.LockGroup([]ResourceLock{lr1})

	assertWaiterWontWait(t, w1)
	assertWaiterIsWaiting(t, w2)

	unlocker := w1.Acquire()

	assertWaiterIsWaiting(t, w2)

	unlocker.Unlock()

	w2.Acquire()
}

func TestLockSpace_ParallelWritesShouldSucced(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "1"})
	w := ls.LockGroup([]ResourceLock{lr1})
	assertWaiterWontWait(t, w)

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2"})
	w = ls.LockGroup([]ResourceLock{lr2})
	assertWaiterWontWait(t, w)
}

func TestLockSpace_ParallelReadsShouldSucced(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1"})
	w := ls.LockGroup([]ResourceLock{lr1})
	assertWaiterWontWait(t, w)

	lr2 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2"})
	w = ls.LockGroup([]ResourceLock{lr2})
	assertWaiterWontWait(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Postfix(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b"})
	ls.LockGroup([]ResourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2"})
	w := ls.LockGroup([]ResourceLock{lr2})

	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Prefix(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2"})
	ls.LockGroup([]ResourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b"})
	w := ls.LockGroup([]ResourceLock{lr2})

	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_AdjacentReadsDoNotBlockEachOther(t *testing.T) {
	ls := NewLockSpaceRun()

	lr1 := NewResourceLock(internal.LockTypeWrite, []string{"a"})
	w1 := ls.LockGroup([]ResourceLock{lr1})

	lr2 := NewResourceLock(internal.LockTypeRead, []string{"a", "b"})
	w2 := ls.LockGroup([]ResourceLock{lr2})

	lr3 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1"})
	w3 := ls.LockGroup([]ResourceLock{lr3})

	lr4 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2"})
	w4 := ls.LockGroup([]ResourceLock{lr4})

	lr5 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1", "a"})
	w5 := ls.LockGroup([]ResourceLock{lr5})

	lr6 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "1", "b"})
	w6 := ls.LockGroup([]ResourceLock{lr6})

	lr7 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2", "a"})
	w7 := ls.LockGroup([]ResourceLock{lr7})

	lr8 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "2", "b"})
	w8 := ls.LockGroup([]ResourceLock{lr8})

	lr9 := NewResourceLock(internal.LockTypeRead, []string{})
	w9 := ls.LockGroup([]ResourceLock{lr9})

	lr10 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2", "b", "c"})
	w10 := ls.LockGroup([]ResourceLock{lr10})

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
	ls := NewLockSpaceRun()

	rl1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	rl2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "2"})
	rl3 := NewResourceLock(internal.LockTypeWrite, []string{"a", "3"})
	w1 := ls.LockGroup([]ResourceLock{rl1, rl2, rl3})

	rl4 := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	rl5 := NewResourceLock(internal.LockTypeRead, []string{"a", "4"})
	rl6 := NewResourceLock(internal.LockTypeRead, []string{"a", "5"})
	w2 := ls.LockGroup([]ResourceLock{rl4, rl5, rl6})

	rl7 := NewResourceLock(internal.LockTypeWrite, []string{"a", "5"})
	rl8 := NewResourceLock(internal.LockTypeWrite, []string{"a", "6"})
	rl9 := NewResourceLock(internal.LockTypeWrite, []string{"a", "7"})
	w3 := ls.LockGroup([]ResourceLock{rl7, rl8, rl9})

	assertWaiterIsWaiting(t, w2)
	assertWaiterIsWaiting(t, w3)

	w1.Acquire().Unlock()

	assertWaiterIsWaiting(t, w3)

	w2.Acquire().Unlock()

	w3.Acquire().Unlock()
}

func TestLockSpace_PartialReadOverlapping(t *testing.T) {
	ls := NewLockSpaceRun()

	rl1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	rl2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "2"})
	rl3 := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	w1 := ls.LockGroup([]ResourceLock{rl1, rl2, rl3})

	rl4 := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	rl5 := NewResourceLock(internal.LockTypeWrite, []string{"a", "4"})
	rl6 := NewResourceLock(internal.LockTypeRead, []string{"a", "5"})
	w2 := ls.LockGroup([]ResourceLock{rl4, rl5, rl6})

	rl7 := NewResourceLock(internal.LockTypeRead, []string{"a", "5"})
	rl8 := NewResourceLock(internal.LockTypeWrite, []string{"a", "6"})
	rl9 := NewResourceLock(internal.LockTypeWrite, []string{"a", "7"})
	w3 := ls.LockGroup([]ResourceLock{rl7, rl8, rl9})

	w3.Acquire()
	w2.Acquire()
	w1.Acquire()
}

func TestLockSpace_HeadAfterTail(t *testing.T) {
	ls := NewLockSpaceRun()

	rl1 := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	w1 := ls.LockGroup([]ResourceLock{rl1})

	rl21 := NewResourceLock(internal.LockTypeRead, []string{"a", "1", "2"})
	rl22 := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	ls.LockGroup([]ResourceLock{rl21, rl22})

	rl3 := NewResourceLock(internal.LockTypeRead, []string{"a", "1"})
	w3 := ls.LockGroup([]ResourceLock{rl3})

	w1.Acquire().Unlock()

	assertWaiterIsWaiting(t, w3)
}

func TestLockSpace_TailAfterHead(t *testing.T) {
	ls := NewLockSpaceRun()

	rl01 := NewResourceLock(internal.LockTypeRead, []string{"a"})
	rl02 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b"})
	ls.LockGroup([]ResourceLock{rl01, rl02})

	rl1 := NewResourceLock(internal.LockTypeRead, []string{"a", "c"})
	w1 := ls.LockGroup([]ResourceLock{rl1})
	assertWaiterWontWait(t, w1)

	rl2 := NewResourceLock(internal.LockTypeRead, []string{"a"})
	w2 := ls.LockGroup([]ResourceLock{rl2})
	assertWaiterWontWait(t, w2)

	rl3 := NewResourceLock(internal.LockTypeRead, []string{"a", "b"})
	w3 := ls.LockGroup([]ResourceLock{rl3})
	assertWaiterIsWaiting(t, w3)

}

func TestLockSpace_Complex_1(t *testing.T) {
	ls := NewLockSpaceRun()
	order := sliceAppender.NewSliceAppender[int]()

	lr1 := NewResourceLock(internal.LockTypeRead, []string{"a"})
	w1 := ls.LockGroup([]ResourceLock{lr1})

	lr2a := NewResourceLock(internal.LockTypeWrite, []string{"a", "1"})
	lr2b := NewResourceLock(internal.LockTypeWrite, []string{"a", "2"})
	w2 := ls.LockGroup([]ResourceLock{lr2a, lr2b})

	lr3a := NewResourceLock(internal.LockTypeRead, []string{})
	w3a := ls.LockGroup([]ResourceLock{lr3a})

	lr3b := NewResourceLock(internal.LockTypeRead, []string{"a", "3"})
	w3b := ls.LockGroup([]ResourceLock{lr3b})

	wg := sync.WaitGroup{}
	wg.Add(4)

	go func() {
		u := w3b.Acquire()
		order.Append(3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w3a.Acquire()
		order.Append(3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w2.Acquire()
		order.Append(2)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w1.Acquire()
		order.Append(1)
		u.Unlock()

		wg.Done()
	}()

	wg.Wait()

	assertOrder(t, order.Value(), []int{1, 2, 3, 3})
}

func TestLockSpace_Complex_2(t *testing.T) {
	ls := NewLockSpaceRun()
	order := sliceAppender.NewSliceAppender[int]()

	wg := sync.WaitGroup{}
	wg.Add(10)

	// 1
	r1 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "c", "d"})
	w1 := ls.LockGroup([]ResourceLock{r1})

	// 2
	r2 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b"})
	w2 := ls.LockGroup([]ResourceLock{r2})

	// 3 ...
	r3 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "1", "a"})
	w3 := ls.LockGroup([]ResourceLock{r3})

	r4 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "1", "b"})
	w4 := ls.LockGroup([]ResourceLock{r4})

	r5 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2", "a"})
	w5 := ls.LockGroup([]ResourceLock{r5})

	r6 := NewResourceLock(internal.LockTypeWrite, []string{"a", "b", "2", "b"})
	w6 := ls.LockGroup([]ResourceLock{r6})

	// 4 ...
	r7 := NewResourceLock(internal.LockTypeRead, []string{})
	w7 := ls.LockGroup([]ResourceLock{r7})

	r8 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "3"})
	w8 := ls.LockGroup([]ResourceLock{r8})

	r9 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "3", "a"})
	w9 := ls.LockGroup([]ResourceLock{r9})

	r10 := NewResourceLock(internal.LockTypeRead, []string{"a", "b", "3", "a", "b"})
	w10 := ls.LockGroup([]ResourceLock{r10})

	go func() {
		u := w10.Acquire()
		order.Append(4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w9.Acquire()
		order.Append(4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w8.Acquire()
		order.Append(4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w7.Acquire()
		order.Append(4)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w6.Acquire()
		order.Append(3)
		u.Unlock()

		fmt.Println(order)
		wg.Done()
	}()

	go func() {
		u := w5.Acquire()
		order.Append(3)
		u.Unlock()

		fmt.Println(order)
		wg.Done()
	}()

	go func() {
		u := w4.Acquire()
		order.Append(3)
		u.Unlock()

		fmt.Println(order)
		wg.Done()
	}()

	go func() {
		u := w3.Acquire()
		order.Append(3)
		u.Unlock()

		fmt.Println(order)
		wg.Done()
	}()

	go func() {
		u := w2.Acquire()
		order.Append(2)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w1.Acquire()
		order.Append(1)
		u.Unlock()

		wg.Done()
	}()

	wg.Wait()

	assertOrder(t, order.Value(), []int{1, 2, 3, 3, 3, 3, 4, 4, 4, 4})
}

func TestStop_StopAfterStop(t *testing.T) {
	ls := NewLockSpaceRun()

	_ = ls

	ls.Stop()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic")
		}
	}()

	ls.Stop()
}

func TestStop_LockGroupAfterStop(t *testing.T) {
	ls := NewLockSpaceRun()

	ls.Stop()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic")
		}
	}()

	ls.LockGroup([]ResourceLock{NewResourceLock(LockTypeRead, []string{"a"})})
}

func TestLockSpace_GroupID(t *testing.T) {
	ls := NewLockSpaceRun()

	w1 := ls.LockGroup([]ResourceLock{NewResourceLock(LockTypeRead, []string{"a"})})
	w2 := ls.LockGroup([]ResourceLock{NewResourceLock(LockTypeRead, []string{"a"})})

	if w1.ID() != 1 {
		t.Error("Expected ID = 1")
	}

	if w2.ID() != 2 {
		t.Error("Expected ID = 2")
	}
}

func TestStatistics_LastGroupID(t *testing.T) {
	ls := NewLockSpaceRun()

	lr := NewResourceLock(LockTypeWrite, []string{"a", "b", "c"})
	locker := ls.LockGroup([]ResourceLock{lr})

	if ls.Statistics().LastGroupID != locker.ID() {
		t.Error("Expected LastGroupID = locker.ID()")
	}
}

func TestStatistics_Tokens(t *testing.T) {
	ls := NewLockSpaceRun()

	s0 := ls.Statistics()

	lr0 := NewResourceLock(LockTypeWrite, []string{"a", "b", "a"})
	lr1 := NewResourceLock(LockTypeWrite, []string{"a", "b", "c"})
	locker := ls.LockGroup([]ResourceLock{lr0, lr1})
	u := locker.Acquire()

	s1 := ls.Statistics()

	if s1.TokensTotal != s0.TokensTotal+6 {
		t.Errorf("Wrong TokensTotal after lock")
	}

	if s1.TokensUnique != s0.TokensUnique+3 {
		t.Errorf("Expected TokensUnique after lock")
	}

	u.Unlock()

	s2 := ls.Statistics()

	if s2.TokensTotal != s0.TokensTotal {
		t.Errorf("Expected TokensTotal after unlock")
	}

	if s2.TokensUnique != s0.TokensUnique {
		t.Errorf("Expected TokensUnique after unlock")
	}
}

func TestStatistics_Groups(t *testing.T) {
	ls := NewLockSpaceRun()

	s0 := ls.Statistics()

	if s0.GroupsPending != 0 {
		t.Errorf("Expected GroupsPending = 0, got %d", s0.GroupsPending)
	}

	if s0.GroupsAcquired != 0 {
		t.Errorf("Expected GroupsAcquired = 0, got %d", s0.GroupsAcquired)
	}

	lr0 := NewResourceLock(LockTypeWrite, []string{"a"})
	locker0 := ls.LockGroup([]ResourceLock{lr0})

	s1 := ls.Statistics()

	if s1.GroupsPending != 0 {
		t.Errorf("Expected GroupsPending = 0, got %d", s1.GroupsPending)
	}

	if s1.GroupsAcquired != 1 {
		t.Errorf("Expected GroupsAcquired = 1,got %d", s1.GroupsAcquired)
	}

	lr1 := NewResourceLock(LockTypeRead, []string{"a"})
	locker1 := ls.LockGroup([]ResourceLock{lr1})

	s2 := ls.Statistics()

	if s2.GroupsPending != 1 {
		t.Errorf("Expected GroupsPending = 1, got %d", s2.GroupsPending)
	}

	if s2.GroupsAcquired != 1 {
		t.Errorf("Expected GroupsAcquired = 1, got %d", s2.GroupsAcquired)
	}

	locker0.Acquire().Unlock()
	locker1.Acquire()

	s3 := ls.Statistics()

	if s3.GroupsPending != 0 {
		t.Errorf("Expected GroupsPending = 0, got %d", s3.GroupsPending)
	}

	if s3.GroupsAcquired != 1 {
		t.Errorf("Expected GroupsAcquired = 1, got %d", s3.GroupsAcquired)
	}

	lr2 := NewResourceLock(LockTypeRead, []string{"a"})
	locker2 := ls.LockGroup([]ResourceLock{lr2})

	locker2.Acquire()

	s4 := ls.Statistics()

	if s4.GroupsPending != 0 {
		t.Errorf("Expected GroupsPending = 0, got %d", s4.GroupsPending)
	}

	if s4.GroupsAcquired != 2 {
		t.Errorf("Expected GroupsAcquired = 2, got %d", s4.GroupsAcquired)
	}
}

func TestStatistics_Locks(t *testing.T) {
	ls := NewLockSpaceRun()

	s0 := ls.Statistics()

	if s0.LocksAcquired != 0 {
		t.Errorf("Expected LocksAcquired = 0, got %d", s0.GroupsPending)
	}

	if s0.LocksPending != 0 {
		t.Errorf("Expected LocksPending = 0, got %d", s0.GroupsAcquired)
	}

	lr0 := NewResourceLock(LockTypeWrite, []string{"a", "b"})
	locker0 := ls.LockGroup([]ResourceLock{lr0})

	s1 := ls.Statistics()

	if s1.LocksAcquired != 1 {
		t.Errorf("Expected LocksAcquired = 1, got %d", s1.GroupsPending)
	}

	if s1.LocksPending != 0 {
		t.Errorf("Expected LocksPending = 0, got %d", s1.GroupsAcquired)
	}

	lr1 := NewResourceLock(LockTypeRead, []string{"a", "b"})
	locker1 := ls.LockGroup([]ResourceLock{lr1})

	s2 := ls.Statistics()

	if s2.LocksAcquired != 1 {
		t.Errorf("Expected LocksAcquired = 1, got %d", s2.GroupsPending)
	}

	if s2.LocksPending != 1 {
		t.Errorf("Expected LocksPending = 1, got %d", s2.GroupsAcquired)
	}

	locker0.Acquire().Unlock()
	locker1.Acquire()

	s3 := ls.Statistics()

	if s3.LocksAcquired != 1 {
		t.Errorf("Expected LocksAcquired = 1, got %d", s3.GroupsPending)
	}

	if s3.LocksPending != 0 {
		t.Errorf("Expected LocksPending = 0, got %d", s3.GroupsAcquired)
	}

	locker1.Acquire().Unlock()

	s4 := ls.Statistics()

	if s4.LocksAcquired != 0 {
		t.Errorf("Expected LocksAcquired = 0, got %d", s4.GroupsPending)
	}

	if s4.LocksPending != 0 {
		t.Errorf("Expected LocksPending = 0, got %d", s4.GroupsAcquired)
	}
}

func TestStatistics_LockShadowing_Write(t *testing.T) {
	ls := NewLockSpaceRun()

	lr0 := NewResourceLock(LockTypeWrite, []string{"a"})
	// Shadowed by lr0
	lr1 := NewResourceLock(LockTypeWrite, []string{"a", "b"})
	// Shadowed by lr0
	lr2 := NewResourceLock(LockTypeWrite, []string{"a", "b", "c"})
	ls.LockGroup([]ResourceLock{lr0, lr1, lr2})

	s := ls.Statistics()
	if s.LocksAcquired != 1 {
		t.Errorf("Expected locks: 1, got %d", s.LocksAcquired)
	}
}

func TestStatistics_LockShadowing_WriteAfterRead(t *testing.T) {
	ls := NewLockSpaceRun()

	lr0 := NewResourceLock(LockTypeRead, []string{"a"})
	// Not shadowed by lr0
	lr1 := NewResourceLock(LockTypeWrite, []string{"a", "b"})
	ls.LockGroup([]ResourceLock{lr0, lr1})

	s := ls.Statistics()
	if s.LocksAcquired != 2 {
		t.Errorf("Expected 2 locks, got %d", s.LocksAcquired)
	}
}

func TestStatistics_LockrefCount(t *testing.T) {
	ls := NewLockSpaceRun()

	s0 := ls.Statistics()

	if s0.LockrefCount != 0 {
		t.Errorf("Expected LockrefCount = 0, got %d", s0.LockrefCount)
	}

	lr0 := NewResourceLock(LockTypeRead, []string{"a"})
	lr1 := NewResourceLock(LockTypeWrite, []string{"a", "b"})
	l := ls.LockGroup([]ResourceLock{lr0, lr1})

	s1 := ls.Statistics()
	if s1.LockrefCount == 0 {
		t.Errorf("There should be lock refs")
	}

	l.Acquire().Unlock()

	ls.Stop()

	s2 := ls.Statistics()
	if s2.LockrefCount != 0 {
		t.Errorf("Expected LockrefCount = 0, got %d", s2.LockrefCount)
	}
}

func TestStop_PathCount(t *testing.T) {
	ls := NewLockSpaceRun()

	lr := NewResourceLock(LockTypeRead, []string{"0"})
	lr1 := NewResourceLock(LockTypeRead, []string{})

	N := 10000
	wg := sync.WaitGroup{}

	for j := 0; j < N; j++ {
		wg.Add(1)

		l := ls.LockGroup([]ResourceLock{lr})
		l1 := ls.LockGroup([]ResourceLock{lr1})

		l.Acquire()
		l.Acquire().Unlock()

		go func() {
			l1.Acquire()
			l1.Acquire().Unlock()

			wg.Done()
		}()
	}

	wg.Wait()

	ls.Stop()

	s := ls.Statistics()

	if s.PathCount != 0 {
		t.Errorf("Expected PathCount = 0, got %d", s.PathCount)
	}
}
