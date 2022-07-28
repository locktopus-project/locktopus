package lockqueue

type GroupLocker struct {
	ch chan Unlocker
	u  Unlocker
}

// Acquire returns when the lock is acquired.
// You may think of it as the casual method Lock from sync.Mutex.
// The reason why the name differs is that the lock actually starts its lifecycle within LockGroup call.
// Use the returned value to unlock the group.
func (l GroupLocker) Acquire() Unlocker {
	<-l.ch

	return l.u
}

func (l GroupLocker) makeReady(u Unlocker) {
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

// Unlock unlocks to group. After returns after the descendent Locker is ready to be acquired.
func (u Unlocker) Unlock() {
	ch := make(chan struct{})
	u.ch <- ch
	<-ch
	close(u.ch)
}
