package main

import (
	ml "github.com/xshkut/distributed-lock/pgk/multilocker"
)

var m = make(map[string]*ml.LockSpace)

func getMultilockerInstance(name string) *ml.LockSpace {
	var ls *ml.LockSpace
	var ok bool

	if ls, ok = m[name]; !ok {
		ls = ml.NewLockSpace()
		m[name] = ls
	}

	return ls
}
