package namespace

import (
	"sync"

	ml "github.com/locktopus-project/locktopus/pkg/multilocker"
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

func GetNamespace(name string) (*ml.MultiLocker, bool) {
	var ns *ml.MultiLocker
	var ok bool

	mx.Lock()
	defer mx.Unlock()

	if ns, ok = namespaces[name]; !ok {
		ns = ml.NewMultilocker()
		namespaces[name] = ns
	}

	return ns, !ok
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

func GetNamespaceStatistics(name string) *ml.MultilockerStatistics {
	mx.Lock()
	defer mx.Unlock()

	if ns, ok := namespaces[name]; ok {
		stats := ns.Statistics()
		return &stats
	}

	return nil
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
