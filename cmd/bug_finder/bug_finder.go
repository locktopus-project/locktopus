// This program is used to find bugs in multilocker.
// It runs forever. Panic is considered to be a bug.

package main

import (
	"fmt"
	"math/rand"
	"time"

	ml "github.com/locktopus-project/locktopus/pkg/multilocker"
)

const branchingFactor = 3
const maxTreeDepth = 3
const fullRangeLockPeriod = 10
const maxGroupSize = 3
const concurrency = 1000
const maxLockDurationMs = 1

func main() {
	ch := make(chan struct{}, 1000)

	fmt.Println("Starting simulation")

	for i := 0; i < concurrency; i++ {
		go func() {
			simulateClient(ch)
		}()
	}

	for {
		ch <- struct{}{}
	}
}

func simulateClient(ch chan struct{}) {
	for range ch {
		resources1 := getRandomResourceLockGroup()
		resources2 := getRandomResourceLockGroup()

		multilocker := ml.NewMultilocker()
		m := NewResourceMap()
		unlocker := make(chan struct{})

		go lockAndCheckCollision(resources1, multilocker, m, unlocker)
		go lockAndCheckCollision(resources2, multilocker, m, unlocker)

		if maxLockDurationMs > 0 {
			time.Sleep(time.Duration(maxLockDurationMs) * time.Millisecond)
		}

		unlocker <- struct{}{}
		unlocker <- struct{}{}
		multilocker.Close()

		multilocker = ml.NewMultilocker()
		m = NewResourceMap()
		unlocker = make(chan struct{})

		go lockAndCheckCollision(resources2, multilocker, m, unlocker)
		go lockAndCheckCollision(resources1, multilocker, m, unlocker)

		if maxLockDurationMs > 0 {
			time.Sleep(time.Duration(maxLockDurationMs) * time.Millisecond)
		}

		unlocker <- struct{}{}
		unlocker <- struct{}{}
		multilocker.Close()
	}
}

func lockAndCheckCollision(resources []ml.ResourceLock, multilocker *ml.MultiLocker, m *ResourceMap, unlock <-chan struct{}) {
	groupRef := new(int8)

	lock := multilocker.Lock(resources)
	u := lock.Acquire()
	defer func() {
		u.Unlock()
	}()

	for _, this := range resources {
		thisPath := this.Path
		thisLockType := this.LockType

		rr := resourceRef{
			group:     groupRef,
			t:         thisLockType,
			resources: resources,
		}

		for i := range this.Path {
			isHead := false
			if i == len(this.Path)-1 {
				isHead = true
			}

			path := ""

			part := this.Path[0 : i+1]
			for j, p := range part {
				if j == len(part)-1 {
					path += p
					continue
				}
				path += p + ":"
			}

			if that, ok := m.Get(path); ok {
				if that.group != groupRef && !(that.t == ml.LockTypeRead && thisLockType == ml.LockTypeRead) {
					panic(fmt.Sprintln("Collision! A:", that.resources, ", B:", rr.resources, ", collision on B:", thisPath))
				}

				continue
			}

			if isHead {
				m.Set(path, rr)
				defer func() {
					m.Delete(path)
				}()
			}
		}
	}

	<-unlock
}

func getRandomResourceName(r int) string {
	return fmt.Sprintf("%d", rand.Intn(r))
}

func getRandomResourcePath() []string {
	result := make([]string, 0)

	l := rand.Intn(maxTreeDepth) + 1

	for i := 0; i < l; i++ {
		if fullRangeLockPeriod != 0 && rand.Intn(fullRangeLockPeriod) == 0 {
			return result
		}

		name := getRandomResourceName(branchingFactor)
		result = append(result, name)
	}

	return result
}

func getRandomResourceLength() int {
	return rand.Intn(maxGroupSize) + 1
}

func getRandomLockType() ml.LockType {
	return ml.LockType(rand.Intn(2))
}

func getRandomResourceLockGroup() []ml.ResourceLock {
	result := make([]ml.ResourceLock, 0)

	for i := 0; i < getRandomResourceLength(); i++ {
		path := getRandomResourcePath()
		lockType := getRandomLockType()

		result = append(result, ml.NewResourceLock(lockType, path))
	}

	return result
}
