package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	lockSpace "github.com/xshkut/distributed-lock/pgk/lock_queue"
)

const branchingFactor = 100
const maxTreeDepth = 5
const fullRangeLockPeriod = 1000
const maxGroupSize = 5
const concurrency = 2000
const maxLockDurationMs = 50

const statsPeriodSec = 1
const endAfterSec = 10

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func init() {
	flag.Parse()
}

func main() {
	var fm *os.File
	startTime := time.Now()
	end := time.After(time.Second * endAfterSec)

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}

		fm, err = os.Create(*cpuprofile + ".mem")
		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	ch := make(chan struct{})

	var expectedRate = float64(float64(concurrency) / float64(maxLockDurationMs) * 1000 * 2)

	ls := lockSpace.NewLockSpace()

	for i := 0; i < concurrency; i++ {
		go simulateClient(ch, ls)
	}

	fmt.Println("GOMAXPROCS:", runtime.GOMAXPROCS(0))
	fmt.Println("Expected rate:", int(expectedRate), "per second")

	time.Sleep(time.Duration(1) * time.Second)

	keepRunning := true
	needPrintStats := false
	lastTime := time.Now()
	lastCount := int64(0)

	printStatsAfter := time.After(time.Duration(statsPeriodSec) * time.Second)

	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)

	initialMemUsage := memStats.Sys

	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

		sig := <-signals
		if sig == syscall.SIGTERM || sig == syscall.SIGINT {
			keepRunning = false
		}
		fmt.Println("Received signal:", sig)
	}()

	for keepRunning {
		ch <- struct{}{}

		select {
		case <-end:
			keepRunning = false
		case <-printStatsAfter:
			needPrintStats = true
			printStatsAfter = time.After(time.Duration(statsPeriodSec) * time.Second)
		default:
		}

		if needPrintStats {
			now := time.Now()

			stats := ls.Statistics()

			memStats := runtime.MemStats{}
			runtime.ReadMemStats(&memStats)

			bytesPerClient := int((memStats.Sys - initialMemUsage) / uint64(concurrency))
			takenTime := now.Sub(lastTime)
			count := stats.LastGroupID - lastCount
			rate := float64(count) / takenTime.Seconds()
			performance := rate / expectedRate * 100

			fmt.Println(stats.LastGroupID, "locks simulated. Taken time:", takenTime.String(), ". Rate =", int(rate), "groups/sec", ". Performance =", fmt.Sprintf("%.2f", performance), "%", ". Bytes per client =", ByteCountIEC(int64(bytesPerClient)))
			fmt.Printf("%+v\n", stats)
			pprof.Lookup("heap").WriteTo(fm, 1)

			lastTime = now
			lastCount = stats.LastGroupID

			needPrintStats = false
		}

	}

	ls.Close()

	stats := ls.Statistics()
	fmt.Printf("%+v\n", stats)

	fmt.Println("Total average rate =", int(float64(stats.LastGroupID)/float64(time.Since(startTime).Seconds())), "groups/sec")
}

func simulateClient(ch chan struct{}, ls *lockSpace.LockSpace) {
	for i := range ch {
		resources := getRandomResourceLockGroup()
		duration := getRandomDuration()

		simulateLock(resources, ls, duration)

		_ = i
	}
}

func getRandomDuration() time.Duration {
	return time.Duration(rand.Intn(maxLockDurationMs)) * time.Millisecond
}

func simulateLock(resources []lockSpace.ResourceLock, ls *lockSpace.LockSpace, duration time.Duration) {
	lock := ls.Lock(resources)
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
