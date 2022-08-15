package multilocker

type Lock struct {
	ch chan struct{}
	u  Unlocker
	id int64
}

// Acquire waits until the lock is acquired and returns corresponding Unlocker.
// You may think of it as the casual method Lock from sync.Mutex.
// The reason why the name differs is that the lock actually starts its lifecycle within LockSpace.Lock() call.
// Use the returned value to unlock the group.
// It is possible to call Acquire() multiple times.
func (l Lock) Acquire() Unlocker {
	<-l.ch

	return l.u
}

// Ready returns chan that signals when l is ready to be acquired.
// It is safe to call Ready() multiple times.
func (l Lock) Ready() <-chan struct{} {
	return l.ch
}

// ID returns unique incremental ID of the group within the LockSpace instance
func (l Lock) ID() int64 {
	return l.id
}

func (l *Lock) makeReady(u Unlocker) {
	l.u = u
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

// Unlock unlocks to Lock and returns after it is completely unlocked, though the descendent Lock is no guaranteed to be ready at that time.
// Do not call Unlock() multiple times.
func (u Unlocker) Unlock() {
	unlockProcessed := make(chan struct{})
	u.ch <- unlockProcessed
	<-unlockProcessed
	close(u.ch)
}
