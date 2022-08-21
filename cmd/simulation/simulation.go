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

	ml "github.com/xshkut/distributed-lock/pgk/multilocker"
)

const branchingFactor = 5
const maxTreeDepth = 5
const fullRangeLockPeriod = 1000
const maxGroupSize = 5
const concurrency = 1000
const maxLockDurationMs = 1

const statsPeriodSec = 60
const endAfterSec = 301

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func init() {
	flag.Parse()
}

func main() {
	var fm *os.File

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

	ch := make(chan struct{}, 1000)

	ls := ml.NewMultilocker()

	for i := 0; i < concurrency; i++ {
		go func() {
			simulateClient(ch)
		}()
	}

	fmt.Println("GOMAXPROCS:", runtime.GOMAXPROCS(0))

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

	startTime := time.Now()
	end := time.After(time.Second * endAfterSec)

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

			fmt.Println(stats.LastGroupID, "locks simulated. Taken time:", takenTime.String(), ". Rate =", int(rate), "groups/sec", ". Bytes per client =", ByteCountIEC(int64(bytesPerClient)))
			fmt.Printf("%+v\n", stats)
			pprof.Lookup("heap").WriteTo(fm, 1)

			lastTime = now
			lastCount = stats.LastGroupID

			needPrintStats = false
		}

	}

	stats := ls.Statistics()
	fmt.Println("Total average rate =", int(float64(stats.LastGroupID)/float64(time.Since(startTime).Seconds())), "groups/sec")

	ls.Close()

	time.Sleep(time.Duration(1) * time.Second)

	fmt.Printf("%+v\n", stats)

}

func simulateClient(ch chan struct{}) {
	for range ch {
		resources1 := getRandomResourceLockGroup()
		resources2 := getRandomResourceLockGroup()

		ls := ml.NewMultilocker()
		m := NewResourceMap()
		unlocker := make(chan struct{})

		go lockAndCheckCollision(resources1, ls, m, unlocker)
		go lockAndCheckCollision(resources2, ls, m, unlocker)

		if maxLockDurationMs > 0 {
			time.Sleep(time.Duration(maxLockDurationMs) * time.Millisecond)
		}

		unlocker <- struct{}{}
		unlocker <- struct{}{}
		ls.Close()

		ls = ml.NewMultilocker()
		m = NewResourceMap()
		unlocker = make(chan struct{})

		go lockAndCheckCollision(resources2, ls, m, unlocker)
		go lockAndCheckCollision(resources1, ls, m, unlocker)

		if maxLockDurationMs > 0 {
			time.Sleep(time.Duration(maxLockDurationMs) * time.Millisecond)
		}

		unlocker <- struct{}{}
		unlocker <- struct{}{}
		ls.Close()
	}
}

func lockAndCheckCollision(resources []ml.ResourceLock, ls *ml.MultiLocker, m *ResourceMap, unlock <-chan struct{}) {
	groupRef := new(int8)

	lock := ls.Lock(resources)
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
