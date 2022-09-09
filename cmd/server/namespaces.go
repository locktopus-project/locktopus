package main

import (
	"sync"

	ml "github.com/xshkut/gearlock/pkg/multilocker"
)

var namespaces = make(map[string]*ml.MultiLocker)

func getMultilockerInstance(name string) *ml.MultiLocker {
	var ls *ml.MultiLocker
	var ok bool

	if ls, ok = namespaces[name]; !ok {
		ls = ml.NewMultilocker()
		namespaces[name] = ls

		mainLogger.Infof("Created new multilocker namespace %s", name)
	}

	return ls
}

func printStatistics() {
	for name, ml := range namespaces {
		stats := ml.Statistics()
		mainLogger.Infof("Multilocker namespace: %s. Statistics: %+v", name, stats)
	}
}

func closeNamespaces() <-chan struct{} {
	ch := make(chan struct{})

	wg := sync.WaitGroup{}
	for _, m := range namespaces {
		wg.Add(1)
		go func(m *ml.MultiLocker) {
			m.Close()
			wg.Done()
		}(m)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}
