package multilocker

type Lock struct {
	ch chan Unlocker
	u  Unlocker
	id int64
}

// Acquire returns when the lock is acquired.
// You may think of it as the casual method Lock from sync.Mutex.
// The reason why the name differs is that the lock actually starts its lifecycle within LockGroup() call.
// Use the returned value to unlock the group.
// It is possible to call Acquire() multiple times.
func (l Lock) Acquire() Unlocker {
	<-l.ch

	return l.u
}

func (l Lock) AcquireChan() <-chan Unlocker {
	return l.ch
}

// ID returns unique incremental ID of the group within the LockSpace
func (l Lock) ID() int64 {
	return l.id
}

func (l Lock) makeReady(u Unlocker) {
	l.u = u
	l.ch <- l.u
	close(l.ch)
}

type Unlocker struct {
	ch chan chan struct{}
}

func NewUnlocker() Unlocker {
	return Unlocker{
		ch: make(chan chan struct{}),
	}
}

// Unlock unlocks to group. After returns after the descendent Locker is ready to be acquired
func (u Unlocker) Unlock() {
	ch := make(chan struct{})
	u.ch <- ch
	<-ch
	close(u.ch)
}
