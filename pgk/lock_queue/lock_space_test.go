package internal

import (
	"testing"

	internal "github.com/xshkut/distributed-lock/pgk/dag_lock"
)

func TestLockSpace_SingleGroupShouldBeLockedImmediately(t *testing.T) {
	ls := NewLockSpace()

	lr := NewLease(internal.LockTypeRead, []string{"a", "b", "c"})

	ls.LockGroup([]lease{lr}, make(chan struct{}))
}

func TestLockSpace_DuplicateRecordsShouldBeMerged(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	lr2 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})

	ls.LockGroup([]lease{lr1, lr2}, make(chan struct{}))
}

func TestLockSpace_FirstGroupShouldBeLockedImmediately(t *testing.T) {
	ls := NewLockSpace()

	lr1 := NewLease(internal.LockTypeWrite, []string{"a", "b", "c"})
	<-ls.LockGroup([]lease{lr1}, make(chan struct{}))
}
