package internal

import (
	"testing"

	internal "github.com/xshkut/distributed-lock/pgk/dag_lock"
)

func TestLockSpace_Sandbox(t *testing.T) {
	ch := make(chan struct{})

	close(ch)
	<-ch
}

func TestLockSpace_SecondArgumentIsReceivedFromChan(t *testing.T) {
	ls := NewLockSpace()

	lr := NewLease(internal.LockTypeRead, []string{"a", "b", "c"})

	ul := NewUnlocker()
	unlock := ls.LockGroup([]lease{lr}, ul).Wait()

	if unlock != ul {
		t.Error("Second argument should be received from chan")
	}
}

func TestLockSpace_SingleGroupShouldBeLockedImmediately(t *testing.T) {
	ls := NewLockSpace()

	lr := NewLease(internal.LockTypeRead, []string{"a", "b", "c"})

	ls.LockGroup([]lease{lr})
}

func TestLockSpace_DuplicateRecordsShouldNotBringDeadlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})

	ls.LockGroup([]lease{lr1, lr2})
}

func TestLockSpace_ConcurrentGroupShouldBlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	ls.LockGroup([]lease{lr1})

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})

	w := ls.LockGroup([]lease{lr2})
	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_EmptyPathShouldAlsoCauseBlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{})
	ls.LockGroup([]lease{lr1})

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	w := ls.LockGroup([]lease{lr2})
	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_TestRelease(t *testing.T) {
	ls := NewLockSpace()

	path := []string{"a", "b", "c"}

	lr1 := NewLease(internal.LockTypeWrite, path)
	w1 := ls.LockGroup([]lease{lr1})
	w2 := ls.LockGroup([]lease{lr1})

	assertWaiterWontWait(t, w1)
	assertWaiterIsWaiting(t, w2)

	unlocker := w1.Wait()

	assertWaiterIsWaiting(t, w2)

	unlocker.Unlock()

	w2.Wait()
}

func TestLockSpace_ParallelWritesShouldSucced(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "1"})
	w := ls.LockGroup([]lease{lr1})
	assertWaiterWontWait(t, w)

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "2"})
	w = ls.LockGroup([]lease{lr2})
	assertWaiterWontWait(t, w)
}

func TestLockSpace_ParallelReadsShouldSucced(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeRead, []string{"a", "b", "1"})
	w := ls.LockGroup([]lease{lr1})
	assertWaiterWontWait(t, w)

	lr2 := NewLease(internal.LockTypeRead, []string{"a", "b", "2"})
	w = ls.LockGroup([]lease{lr2})
	assertWaiterWontWait(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Postfix(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b"})
	ls.LockGroup([]lease{lr1})

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "2"})
	w := ls.LockGroup([]lease{lr2})

	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_SequentialWritesShouldBlocked_Prefix(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "2"})
	ls.LockGroup([]lease{lr1})

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b"})
	w := ls.LockGroup([]lease{lr2})

	assertWaiterIsWaiting(t, w)
}

func TestLockSpace_Complex_1(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "2"})
	ls.LockGroup([]lease{lr1})

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b"})
	w := ls.LockGroup([]lease{lr2})
	assertWaiterIsWaiting(t, w)
}

func assertWaiterIsWaiting(t *testing.T, lw LockWaiter) {
	select {
	case <-lw.ch:
		t.Error("Waiter should still wait")
	default:
	}
}

func assertWaiterWontWait(t *testing.T, lw LockWaiter) {
	select {
	case <-lw.ch:
	default:
		t.Error("Waiter should have completed")
	}
}
