package multilocker

import (
	"encoding/binary"
	"unsafe"
)

type token uintptr

const tokenSizeBytes = int(unsafe.Sizeof(uintptr(0)))

func newTokenBuffer(size int) []byte {
	return make([]byte, size*tokenSizeBytes)
}

func concatTokenRefs(pointers []token, buffer []byte) string {
	for i, p := range pointers {
		binary.PutVarint(buffer[i*tokenSizeBytes:], int64(p))
	}

	return string(buffer[0 : len(pointers)*tokenSizeBytes])
}
