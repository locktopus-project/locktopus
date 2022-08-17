package multilocker_test

import (
	"reflect"
	"sync"
	"testing"

	ml "github.com/xshkut/distributed-lock/pgk/multilocker"
	sliceAppender "github.com/xshkut/distributed-lock/pgk/slice_appender"
)

func assertLockIsWaiting(t *testing.T, lw ml.Lock) {
	select {
	case <-lw.Ready():
		t.Error("Lock should still wait")
	default:
	}
}

func assertLockWontWait(t *testing.T, lw ml.Lock) {
	select {
	case <-lw.Ready():
	default:
		t.Error("Lock should not wait for acquiring")
	}
}

func assertOrder(t *testing.T, order []int, expected []int) {
	if !reflect.DeepEqual(order, expected) {
		t.Errorf("Order is %v, expected %v", order, expected)
	}
}

func TestLockSpace_SecondArgumentIsReceivedFromChan(t *testing.T) {
	ls := ml.NewLockSpace()

	lr := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "c"})

	ul := ml.NewUnlocker()
	unlock := ls.Lock([]ml.ResourceLock{lr}, ul).Acquire()

	if unlock != ul {
		t.Error("Second argument should be received from chan")
	}
}

func TestLockSpace_SingleGroupShouldBeLockedImmediately(t *testing.T) {
	ls := ml.NewLockSpace()

	lr := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "c"})

	ls.Lock([]ml.ResourceLock{lr})
}

func TestLockSpace_DuplicateRecordsShouldNotBringDeadlock(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})
	lr2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})

	ls.Lock([]ml.ResourceLock{lr1, lr2})
}

func TestLockSpace_ConcurrentGroupShouldBlock(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})
	ls.Lock([]ml.ResourceLock{lr1})

	lr2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})

	w := ls.Lock([]ml.ResourceLock{lr2})
	assertLockIsWaiting(t, w)
}

func TestLockSpace_EmptyPathShouldAlsoCauseBlock(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{})
	ls.Lock([]ml.ResourceLock{lr1})

	lr2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})
	w := ls.Lock([]ml.ResourceLock{lr2})
	assertLockIsWaiting(t, w)
}

func TestLockSpace_TestRelease(t *testing.T) {
	ls := ml.NewLockSpace()

	path := []string{"a", "b", "c"}

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, path)
	w1 := ls.Lock([]ml.ResourceLock{lr1})
	w2 := ls.Lock([]ml.ResourceLock{lr1})

	assertLockWontWait(t, w1)
	assertLockIsWaiting(t, w2)

	unlocker := w1.Acquire()

	assertLockIsWaiting(t, w2)

	unlocker.Unlock()

	w2.Acquire()
}

func TestLockSpace_ParallelWritesShouldSucced(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "1"})
	w := ls.Lock([]ml.ResourceLock{lr1})
	assertLockWontWait(t, w)

	lr2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "2"})
	w = ls.Lock([]ml.ResourceLock{lr2})
	assertLockWontWait(t, w)
}

func TestLockSpace_ParallelReadsShouldSucced(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "1"})
	w := ls.Lock([]ml.ResourceLock{lr1})
	assertLockWontWait(t, w)

	lr2 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "2"})
	w = ls.Lock([]ml.ResourceLock{lr2})
	assertLockWontWait(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Postfix(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	ls.Lock([]ml.ResourceLock{lr1})

	lr2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "2"})
	w := ls.Lock([]ml.ResourceLock{lr2})

	assertLockIsWaiting(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Prefix(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "2"})
	ls.Lock([]ml.ResourceLock{lr1})

	lr2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	w := ls.Lock([]ml.ResourceLock{lr2})

	assertLockIsWaiting(t, w)
}

func TestLockSpace_AdjacentReadsDoNotBlockEachOther(t *testing.T) {
	ls := ml.NewLockSpace()

	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})
	w1 := ls.Lock([]ml.ResourceLock{lr1})

	lr2 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b"})
	w2 := ls.Lock([]ml.ResourceLock{lr2})

	lr3 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "1"})
	w3 := ls.Lock([]ml.ResourceLock{lr3})

	lr4 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "2"})
	w4 := ls.Lock([]ml.ResourceLock{lr4})

	lr5 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "1", "a"})
	w5 := ls.Lock([]ml.ResourceLock{lr5})

	lr6 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "1", "b"})
	w6 := ls.Lock([]ml.ResourceLock{lr6})

	lr7 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "2", "a"})
	w7 := ls.Lock([]ml.ResourceLock{lr7})

	lr8 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "2", "b"})
	w8 := ls.Lock([]ml.ResourceLock{lr8})

	lr9 := ml.NewResourceLock(ml.LockTypeRead, []string{})
	w9 := ls.Lock([]ml.ResourceLock{lr9})

	lr10 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "2", "b", "c"})
	w10 := ls.Lock([]ml.ResourceLock{lr10})

	assertLockWontWait(t, w1)

	assertLockIsWaiting(t, w2)
	assertLockIsWaiting(t, w3)
	assertLockIsWaiting(t, w4)
	assertLockIsWaiting(t, w5)
	assertLockIsWaiting(t, w6)
	assertLockIsWaiting(t, w7)
	assertLockIsWaiting(t, w8)
	assertLockIsWaiting(t, w9)

	assertLockIsWaiting(t, w10)

	u := w1.Acquire()

	assertLockIsWaiting(t, w2)
	assertLockIsWaiting(t, w3)
	assertLockIsWaiting(t, w4)
	assertLockIsWaiting(t, w5)
	assertLockIsWaiting(t, w6)
	assertLockIsWaiting(t, w7)
	assertLockIsWaiting(t, w8)
	assertLockIsWaiting(t, w9)

	assertLockIsWaiting(t, w10)

	u.Unlock()

	u9 := w9.Acquire()
	u8 := w8.Acquire()
	u7 := w7.Acquire()
	u6 := w6.Acquire()
	u5 := w5.Acquire()
	u4 := w4.Acquire()
	u3 := w3.Acquire()
	u2 := w2.Acquire()

	assertLockIsWaiting(t, w10)

	for _, u := range []ml.Unlocker{u9, u8, u7, u6, u5, u4, u3, u2} {
		u.Unlock()
	}

	w10.Acquire()
}

func TestLockSpace_PartialWriteOverlapping(t *testing.T) {
	ls := ml.NewLockSpace()

	rl1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "1"})
	rl2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "2"})
	rl3 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "3"})
	w1 := ls.Lock([]ml.ResourceLock{rl1, rl2, rl3})

	rl4 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "3"})
	rl5 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "4"})
	rl6 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "5"})
	w2 := ls.Lock([]ml.ResourceLock{rl4, rl5, rl6})

	rl7 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "5"})
	rl8 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "6"})
	rl9 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "7"})
	w3 := ls.Lock([]ml.ResourceLock{rl7, rl8, rl9})

	assertLockIsWaiting(t, w2)
	assertLockIsWaiting(t, w3)

	w1.Acquire().Unlock()

	assertLockIsWaiting(t, w3)

	w2.Acquire().Unlock()

	w3.Acquire().Unlock()
}

func TestLockSpace_PartialReadOverlapping(t *testing.T) {
	ls := ml.NewLockSpace()

	rl1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "1"})
	rl2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "2"})
	rl3 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "3"})
	w1 := ls.Lock([]ml.ResourceLock{rl1, rl2, rl3})

	rl4 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "3"})
	rl5 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "4"})
	rl6 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "5"})
	w2 := ls.Lock([]ml.ResourceLock{rl4, rl5, rl6})

	rl7 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "5"})
	rl8 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "6"})
	rl9 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "7"})
	w3 := ls.Lock([]ml.ResourceLock{rl7, rl8, rl9})

	w3.Acquire()
	w2.Acquire()
	w1.Acquire()
}

func TestLockSpace_HeadAfterTail(t *testing.T) {
	ls := ml.NewLockSpace()

	rl1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "1"})
	w1 := ls.Lock([]ml.ResourceLock{rl1})

	rl21 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "1", "2"})
	rl22 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "1"})
	ls.Lock([]ml.ResourceLock{rl21, rl22})

	rl3 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "1"})
	w3 := ls.Lock([]ml.ResourceLock{rl3})

	w1.Acquire().Unlock()

	assertLockIsWaiting(t, w3)
}

func TestLockSpace_TailAfterHead(t *testing.T) {
	ls := ml.NewLockSpace()

	rl01 := ml.NewResourceLock(ml.LockTypeRead, []string{"a"})
	rl02 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	ls.Lock([]ml.ResourceLock{rl01, rl02})

	rl1 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "c"})
	w1 := ls.Lock([]ml.ResourceLock{rl1})
	assertLockWontWait(t, w1)

	rl2 := ml.NewResourceLock(ml.LockTypeRead, []string{"a"})
	w2 := ls.Lock([]ml.ResourceLock{rl2})
	assertLockWontWait(t, w2)

	rl3 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b"})
	w3 := ls.Lock([]ml.ResourceLock{rl3})
	assertLockIsWaiting(t, w3)

}

func TestLockSpace_Complex_1(t *testing.T) {
	ls := ml.NewLockSpace()
	order := sliceAppender.NewSliceAppender[int]()

	lr1 := ml.NewResourceLock(ml.LockTypeRead, []string{"a"})
	w1 := ls.Lock([]ml.ResourceLock{lr1})

	lr2a := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "1"})
	lr2b := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "2"})
	w2 := ls.Lock([]ml.ResourceLock{lr2a, lr2b})

	lr3a := ml.NewResourceLock(ml.LockTypeRead, []string{})
	w3a := ls.Lock([]ml.ResourceLock{lr3a})

	lr3b := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "3"})
	w3b := ls.Lock([]ml.ResourceLock{lr3b})

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
	ls := ml.NewLockSpace()
	order := sliceAppender.NewSliceAppender[int]()

	wg := sync.WaitGroup{}
	wg.Add(10)

	// 1
	r1 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "c", "d"})
	w1 := ls.Lock([]ml.ResourceLock{r1})

	// 2
	r2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	w2 := ls.Lock([]ml.ResourceLock{r2})

	// 3 ...
	r3 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "1", "a"})
	w3 := ls.Lock([]ml.ResourceLock{r3})

	r4 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "1", "b"})
	w4 := ls.Lock([]ml.ResourceLock{r4})

	r5 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "2", "a"})
	w5 := ls.Lock([]ml.ResourceLock{r5})

	r6 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "2", "b"})
	w6 := ls.Lock([]ml.ResourceLock{r6})

	// 4 ...
	r7 := ml.NewResourceLock(ml.LockTypeRead, []string{})
	w7 := ls.Lock([]ml.ResourceLock{r7})

	r8 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "3"})
	w8 := ls.Lock([]ml.ResourceLock{r8})

	r9 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "3", "a"})
	w9 := ls.Lock([]ml.ResourceLock{r9})

	r10 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b", "3", "a", "b"})
	w10 := ls.Lock([]ml.ResourceLock{r10})

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

		wg.Done()
	}()

	go func() {
		u := w5.Acquire()
		order.Append(3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w4.Acquire()
		order.Append(3)
		u.Unlock()

		wg.Done()
	}()

	go func() {
		u := w3.Acquire()
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

	assertOrder(t, order.Value(), []int{1, 2, 3, 3, 3, 3, 4, 4, 4, 4})
}

func TestStop_StopAfterStop(t *testing.T) {
	ls := ml.NewLockSpace()

	_ = ls

	ls.Close()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic")
		}
	}()

	ls.Close()
}

func TestStop_LockGroupAfterStop(t *testing.T) {
	ls := ml.NewLockSpace()

	ls.Close()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic")
		}
	}()

	ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeRead, []string{"a"})})
}

func TestLockSpace_GroupID(t *testing.T) {
	ls := ml.NewLockSpace()

	w1 := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeRead, []string{"a"})})
	w2 := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeRead, []string{"a"})})

	if w1.ID() != 1 {
		t.Error("Expected ID = 1")
	}

	if w2.ID() != 2 {
		t.Error("Expected ID = 2")
	}
}

func TestStatistics_LastGroupID(t *testing.T) {
	ls := ml.NewLockSpace()

	lr := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})
	locker := ls.Lock([]ml.ResourceLock{lr})

	if ls.Statistics().LastGroupID != locker.ID() {
		t.Error("Expected LastGroupID = locker.ID()")
	}
}

func TestStatistics_Tokens(t *testing.T) {
	ls := ml.NewLockSpace()

	s0 := ls.Statistics()

	lr0 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "a"})
	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})
	locker := ls.Lock([]ml.ResourceLock{lr0, lr1})
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
	ls := ml.NewLockSpace()

	s0 := ls.Statistics()

	if s0.GroupsPending != 0 {
		t.Errorf("Expected GroupsPending = 0, got %d", s0.GroupsPending)
	}

	if s0.GroupsAcquired != 0 {
		t.Errorf("Expected GroupsAcquired = 0, got %d", s0.GroupsAcquired)
	}

	lr0 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})
	locker0 := ls.Lock([]ml.ResourceLock{lr0})

	s1 := ls.Statistics()

	if s1.GroupsPending != 0 {
		t.Errorf("Expected GroupsPending = 0, got %d", s1.GroupsPending)
	}

	if s1.GroupsAcquired != 1 {
		t.Errorf("Expected GroupsAcquired = 1,got %d", s1.GroupsAcquired)
	}

	lr1 := ml.NewResourceLock(ml.LockTypeRead, []string{"a"})
	locker1 := ls.Lock([]ml.ResourceLock{lr1})

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

	lr2 := ml.NewResourceLock(ml.LockTypeRead, []string{"a"})
	locker2 := ls.Lock([]ml.ResourceLock{lr2})

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
	ls := ml.NewLockSpace()

	s0 := ls.Statistics()

	if s0.LocksAcquired != 0 {
		t.Errorf("Expected LocksAcquired = 0, got %d", s0.GroupsPending)
	}

	if s0.LocksPending != 0 {
		t.Errorf("Expected LocksPending = 0, got %d", s0.GroupsAcquired)
	}

	lr0 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	locker0 := ls.Lock([]ml.ResourceLock{lr0})

	s1 := ls.Statistics()

	if s1.LocksAcquired != 1 {
		t.Errorf("Expected LocksAcquired = 1, got %d", s1.GroupsPending)
	}

	if s1.LocksPending != 0 {
		t.Errorf("Expected LocksPending = 0, got %d", s1.GroupsAcquired)
	}

	lr1 := ml.NewResourceLock(ml.LockTypeRead, []string{"a", "b"})
	locker1 := ls.Lock([]ml.ResourceLock{lr1})

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
	ls := ml.NewLockSpace()

	lr0 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})
	// Shadowed by lr0
	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	// Shadowed by lr0
	lr2 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b", "c"})
	ls.Lock([]ml.ResourceLock{lr0, lr1, lr2})

	s := ls.Statistics()
	if s.LocksAcquired != 1 {
		t.Errorf("Expected locks: 1, got %d", s.LocksAcquired)
	}
}

func TestStatistics_LockShadowing_WriteAfterRead(t *testing.T) {
	ls := ml.NewLockSpace()

	lr0 := ml.NewResourceLock(ml.LockTypeRead, []string{"a"})
	// Not shadowed by lr0
	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	ls.Lock([]ml.ResourceLock{lr0, lr1})

	s := ls.Statistics()
	if s.LocksAcquired != 2 {
		t.Errorf("Expected 2 locks, got %d", s.LocksAcquired)
	}
}

func TestStatistics_LockrefCount(t *testing.T) {
	ls := ml.NewLockSpace()

	s0 := ls.Statistics()

	if s0.LockrefCount != 0 {
		t.Errorf("Expected LockrefCount = 0, got %d", s0.LockrefCount)
	}

	lr0 := ml.NewResourceLock(ml.LockTypeRead, []string{"a"})
	lr1 := ml.NewResourceLock(ml.LockTypeWrite, []string{"a", "b"})
	l := ls.Lock([]ml.ResourceLock{lr0, lr1})

	s1 := ls.Statistics()
	if s1.LockrefCount == 0 {
		t.Errorf("There should be lock refs")
	}

	l.Acquire().Unlock()

	ls.Close()

	s2 := ls.Statistics()
	if s2.LockrefCount != 0 {
		t.Errorf("Expected LockrefCount = 0, got %d", s2.LockrefCount)
	}
}

func TestStop_PathCount(t *testing.T) {
	ls := ml.NewLockSpace()

	lr := ml.NewResourceLock(ml.LockTypeRead, []string{"0"})
	lr1 := ml.NewResourceLock(ml.LockTypeRead, []string{})

	N := 100
	wg := sync.WaitGroup{}

	for j := 0; j < N; j++ {
		wg.Add(1)

		l := ls.Lock([]ml.ResourceLock{lr})
		l1 := ls.Lock([]ml.ResourceLock{lr1})

		l.Acquire()
		l.Acquire().Unlock()

		go func() {
			l1.Acquire()
			l1.Acquire().Unlock()

			wg.Done()
		}()
	}

	wg.Wait()

	ls.Close()

	s := ls.Statistics()

	if s.PathCount != 0 {
		t.Errorf("Expected PathCount = 0, got %d", s.PathCount)
	}
}

func TestLock_MultipleAcquires(t *testing.T) {
	ls := ml.NewLockSpace()

	l := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})})
	l1 := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})})

	assertLockIsWaiting(t, l1)

	l.Acquire()
	l.Acquire()
	u := l.Acquire()

	u.Unlock()

	l1.Acquire()
}

func TestReady_NoLockers(t *testing.T) {
	ls := ml.NewLockSpace()

	l := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})})

	<-l.Ready()
}

func TestReady_WithLockers(t *testing.T) {
	ls := ml.NewLockSpace()

	l := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})})
	l1 := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})})

	select {
	case <-l1.Ready():
		t.Errorf("Expected l1 not be ready")
	default:
	}

	l.Acquire().Unlock()

	<-l1.Ready()
}

func TestLock_PrefixAfterGroupWithFirstRead(t *testing.T) {
	ls := ml.NewLockSpace()

	l := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeRead, []string{}), ml.NewResourceLock(ml.LockTypeWrite, []string{"c"})})
	l1 := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeRead, []string{})})

	l.Acquire()

	assertLockIsWaiting(t, l1)
}

func TestLock_WriteAfterPostfixRead(t *testing.T) {
	ls := ml.NewLockSpace()

	l := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeRead, []string{"a"})})
	l.Acquire()

	l1 := ls.Lock([]ml.ResourceLock{ml.NewResourceLock(ml.LockTypeRead, []string{"a"}), ml.NewResourceLock(ml.LockTypeWrite, []string{"a"})})

	assertLockIsWaiting(t, l1)
}

func TestLock_Complex_0(t *testing.T) {
	ls := ml.NewLockSpace()

	l := ls.Lock([]ml.ResourceLock{
		ml.NewResourceLock(ml.LockTypeRead, []string{"1"}),
	})
	l.Acquire()

	l1 := ls.Lock([]ml.ResourceLock{
		ml.NewResourceLock(ml.LockTypeRead, []string{"1", "5"}),
		ml.NewResourceLock(ml.LockTypeWrite, []string{"1", "2", "4", "1"}),
	})

	assertLockIsWaiting(t, l1)
}
