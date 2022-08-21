package main

import (
	ml "github.com/xshkut/gearlock/pkg/multilocker"
)

var m = make(map[string]*ml.MultiLocker)

func getMultilockerInstance(name string) *ml.MultiLocker {
	var ls *ml.MultiLocker
	var ok bool

	if ls, ok = m[name]; !ok {
		ls = ml.NewMultilocker()
		m[name] = ls
	}

	return ls
}
