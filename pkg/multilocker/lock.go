package multilocker

type Lock struct {
	ch chan struct{}
	u  Unlocker
	id int64
}

// Acquire waits until Lock is acquired and returns corresponding Unlocker.
// Use the returned value to unlock the group.
// It is ok to call Acquire() multiple times, though it will not have further side effects.
func (l *Lock) Acquire() Unlocker {
	<-l.ch

	return l.u
}

// Ready returns chan that signals when l is ready to be acquired.
// It is safe to call Ready() multiple times.
// Unlike Acquire(), Ready() can be used with "select" statement.
func (l *Lock) Ready() <-chan struct{} {
	return l.ch
}

// ID returns unique incremental ID of the group within the LockSpace instance
func (l *Lock) ID() int64 {
	return l.id
}

func (l *Lock) makeReady(u Unlocker) {
	l.u = u
	close(l.ch)
}

// Unlocker is used to release the lock acquired by Lock().
// Use NewUnlocker() to create new Unlocker.
type Unlocker struct {
	ch chan chan struct{}
}

func NewUnlocker() Unlocker {
	return Unlocker{
		ch: make(chan chan struct{}),
	}
}

// Unlock unlocks Lock and returns after it is completely unlocked, though there is no guarantee that the dependent Lock (if exists) is ready to be acquired at that time.
// Calling Unlock() before the Lock is enqueued can lead to panic.
// Do not call Unlock() more than once.
func (u Unlocker) Unlock() {
	unlockProcessed := make(chan struct{})
	u.ch <- unlockProcessed
	<-unlockProcessed
	close(u.ch)
}
