package internal

import (
	"testing"

	internal "github.com/xshkut/distributed-lock/pgk/dag_lock"
)

func TestLockSpace_SingleGroupShouldBeLockedImmediately(t *testing.T) {
	ls := NewLockSpace()

	lr := NewLease(internal.LockTypeRead, []string{"a", "b", "c"})

	ls.LockGroup([]lease{lr}, make(chan interface{}))
}

func TestLockSpace_DuplicateRecordsShouldNotBringDeadlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})

	ls.LockGroup([]lease{lr1, lr2}, make(chan interface{}))
}

func TestLockSpace_ConcurrentGroupShouldBlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	<-ls.LockGroup([]lease{lr1}, make(chan interface{}))

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	select {
	case <-ls.LockGroup([]lease{lr2}, make(chan interface{})):
		t.Error("Concurrent group should be blocked")
	default:
	}
}

func TestLockSpace_EmptyPathShouldAlsoCauseBlock(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{})
	<-ls.LockGroup([]lease{lr1}, make(chan interface{}))

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	select {
	case <-ls.LockGroup([]lease{lr2}, make(chan interface{})):
		t.Error("Concurrent group should be blocked")
	default:
	}
}

func TestLockSpace_TestRelease(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	unlock := <-ls.LockGroup([]lease{lr1}, make(chan interface{}))
	unlock <- nil

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	<-ls.LockGroup([]lease{lr2}, make(chan interface{}))
}

func TestLockSpace_ParallelWritesShouldSucced(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "1"})
	<-ls.LockGroup([]lease{lr1}, make(chan interface{}))

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "2"})
	<-ls.LockGroup([]lease{lr2}, make(chan interface{}))
}

func TestLockSpace_ParallelReadsShouldSucced(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeRead, []string{"a", "b", "1"})
	<-ls.LockGroup([]lease{lr1}, make(chan interface{}))

	lr2 := NewLease(internal.LockTypeRead, []string{"a", "b", "2"})
	<-ls.LockGroup([]lease{lr2}, make(chan interface{}))
}

func TestLockSpace_SequentialWritesShouldBlocked_Postfix(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b"})
	<-ls.LockGroup([]lease{lr1}, make(chan interface{}))

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "2"})
	select {
	case <-ls.LockGroup([]lease{lr2}, make(chan interface{})):
		t.Error("Sequential writes should be blocked")
	default:
	}
}

func TestLockSpace_SequentialWritesShouldBlocked_Prefix(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "2"})
	<-ls.LockGroup([]lease{lr1}, make(chan interface{}))

	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b"})
	select {
	case <-ls.LockGroup([]lease{lr2}, make(chan interface{})):
		t.Error("Sequential writes should be blocked")
	default:
	}
}
