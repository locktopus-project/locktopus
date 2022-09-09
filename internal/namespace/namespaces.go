package namespace

import (
	"sync"

	ml "github.com/xshkut/gearlock/pkg/multilocker"
)

var namespaces = make(map[string]*ml.MultiLocker)

var mx = sync.Mutex{}

type NamespaceEntry struct {
	Name      string
	Namespace *ml.MultiLocker
}

func GetNamespaces() []NamespaceEntry {
	list := make([]NamespaceEntry, 0, len(namespaces))

	for name, m := range namespaces {
		list = append(list, NamespaceEntry{
			Name:      name,
			Namespace: m,
		})
	}

	return list
}

func GetMultilockerInstance(name string) (*ml.MultiLocker, bool) {
	var ls *ml.MultiLocker
	var ok bool

	mx.Lock()
	defer mx.Unlock()

	if ls, ok = namespaces[name]; !ok {
		ls = ml.NewMultilocker()
		namespaces[name] = ls
	}

	return ls, !ok
}

type NamespaceStatistics struct {
	Name  string
	Stats ml.MultilockerStatistics
}

func GetStatistics() []NamespaceStatistics {
	list := make([]NamespaceStatistics, 0, len(namespaces))

	for name, ml := range namespaces {
		stats := ml.Statistics()

		list = append(list, NamespaceStatistics{
			Name:  name,
			Stats: stats,
		})
	}

	return list
}

func CloseNamespaces() <-chan struct{} {
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
