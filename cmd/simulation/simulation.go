package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"

	lockSpace "github.com/xshkut/distributed-lock/pgk/lock_queue"
)

const branchingFactor = 10
const maxTreeDepth = 10
const fullRangeLockPeriod = 10
const maxGroupSize = 5
const concurrency = 1 * 100000
const maxLockDurationMs = 1

const statsPeriod = 10000

var expectedRate = float64(float64(concurrency) / float64(maxLockDurationMs) * 1000 * 2)

func main() {
	ch := make(chan int)

	ls := lockSpace.NewLockSpace()

	for i := 0; i < concurrency; i++ {
		go simulateClient(ch, ls)
	}

	fmt.Println("GOMAXPROCS:", runtime.GOMAXPROCS(0))
	fmt.Println("Expected rate:", int(expectedRate), "per second")

	time.Sleep(time.Duration(1) * time.Second)

	i := 0
	lastTime := time.Now()
	for {
		ch <- i
		i++

		if i == statsPeriod {
			i = 0
			now := time.Now()

			memStats := runtime.MemStats{}
			runtime.ReadMemStats(&memStats)

			bytesPerClient := int(memStats.Sys / uint64(concurrency))
			takenTime := now.Sub(lastTime)
			rate := float64(statsPeriod) / takenTime.Seconds()
			performance := rate / expectedRate * 100
			fmt.Println(statsPeriod, "locks simulated. Taken time:", takenTime.String(), ". Rate =", int(rate), "locks/sec", ". Performance =", fmt.Sprintf("%.2f", performance), "%", ". Bytes per client =", ByteCountIEC(int64(bytesPerClient)))

			lastTime = now
		}
	}
}

func simulateClient(ch chan int, ls *lockSpace.LockSpace) {
	for i := range ch {
		resources := getRandomResourceLockGroup()
		duration := getRandomDuration()

		// fmt.Println("Simulation lock for resources:", resources, "for", duration, "id:", i)

		simulateLock(resources, ls, duration)

		_ = i
		// fmt.Println("Simulation completed (id =", i, ")")
	}
}

func getRandomDuration() time.Duration {
	return time.Duration(rand.Intn(maxLockDurationMs)) * time.Millisecond * 0
}

func simulateLock(resources []lockSpace.ResourceLock, ls *lockSpace.LockSpace, duration time.Duration) {
	lock := ls.LockGroup(resources)
	u := lock.Acquire()

	if duration > 0 {
		time.Sleep(duration)
	}

	u.Unlock()
}

func getRandomResourceName(r int) string {
	return fmt.Sprintf("%d", rand.Intn(r))
}

func getRandomResourcePath() []string {
	result := make([]string, 0)

	for i := 0; i < maxTreeDepth; i++ {
		if rand.Intn(fullRangeLockPeriod) == 0 {
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

func getRandomLockType() lockSpace.LockType {
	return lockSpace.LockType(rand.Intn(2))
}

func getRandomResourceLockGroup() []lockSpace.ResourceLock {
	result := make([]lockSpace.ResourceLock, 0)

	for i := 0; i < getRandomResourceLength(); i++ {
		path := getRandomResourcePath()
		lockType := getRandomLockType()

		result = append(result, lockSpace.NewResourceLock(lockType, path))
	}

	return result
}

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
