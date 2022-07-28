package lockqueue

type Lock struct {
	ch chan Unlocker
	u  Unlocker
}

// Acquire returns when the lock is acquired.
// You may think of it as the casual method Lock from sync.Mutex.
// The reason why the name differs is that the lock actually starts its lifecycle within LockGroup call.
// Use the returned value to unlock the group.
func (l Lock) Acquire() Unlocker {
	<-l.ch

	return l.u
}

func (l Lock) makeReady(u Unlocker) {
	l.u = u
	l.ch <- l.u
	close(l.ch)
}

type Unlocker struct {
	ch chan struct{}
}

func NewUnlocker() Unlocker {
	return Unlocker{
		ch: make(chan struct{}),
	}
}

func (u Unlocker) Unlock() {
	close(u.ch)
}
