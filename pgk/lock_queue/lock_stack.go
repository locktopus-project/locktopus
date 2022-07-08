package internal

import (
	"encoding/binary"
	"fmt"
	"sync"
	"unsafe"

	chainMutex "github.com/xshkut/distributed-lock/pgk/chain_mutex"
	setCounter "github.com/xshkut/distributed-lock/pgk/set_counter"
)

type lockPath struct {
	lockType     chainMutex.LockType
	pathSegments []string
}

func NewLockPath(lockType chainMutex.LockType, resourcePath []string) lockPath {
	return lockPath{
		lockType:     lockType,
		pathSegments: resourcePath,
	}
}

type LockPoint struct {
	path    string
	isLeaf  bool
	cm      *chainMutex.ChainMutex
	layerID uint64
}

type LockSpace struct {
	mx          sync.Mutex
	nextLayerID uint64
	segments    setCounter.SetCounter
	lockSurface map[string]LockPoint
}

func NewLockSpace() *LockSpace {
	return &LockSpace{
		nextLayerID: 0,
		segments:    setCounter.NewSetCounter(),
		lockSurface: make(map[string]LockPoint),
	}
}

// Lock locks the whole layer. Returns when the layer is locked. Use 'release' to unlock or cancel the lock (even if the lock has not been returned yet).
func (ls *LockSpace) LockLayer(resources []lockPath) {
	// lq.mx.Lock()
	// defer lq.mx.Unlock()
	ls.nextLayerID++

}

func (ls *LockSpace) lockPath(lp lockPath) (cm *chainMutex.ChainMutex) {
	lockPaths := make([]string, len(lp.pathSegments))

	for i, _ := range lp.pathSegments {
		lockPaths[i] = ls.getResourceCompactPath(lp.pathSegments[:i+1])
	}

	maxLayerId := uint64(0)
	var highetsCM *chainMutex.ChainMutex

	for i, path := range lockPaths {
		isLeaf := i == len(lockPaths)-1

		if lp, ok := ls.lockSurface[path]; !ok {
			if !lp.isLeaf && !isLeaf {
				continue
			}

			if lp.layerID > maxLayerId {
				maxLayerId = lp.layerID
				highetsCM = lp.cm
			}
		}
	}

	if highetsCM == nil {
		return chainMutex.NewChainMutex(lp.lockType)
	}

	return highetsCM.Chain(lp.lockType)
}

func (ls *LockSpace) getResourceCompactPath(path []string) string {
	tokenPointers := make([]uintptr, len(path))

	for i, segment := range path {
		p := ls.segments.Store(segment)

		tokenPointers[i] = uintptr(unsafe.Pointer(p))
	}

	return buildCompactPathFromPointers(tokenPointers)
}

func (ls *LockSpace) releaseResourcePath(path []string) {
	for _, segment := range path {
		ls.segments.Release(segment)
	}
}

func buildCompactPathFromPointers(pointers []uintptr) string {
	concatSlice := make([]byte, 0)

	for _, p := range pointers {
		bs := uintptrToBytes(p)
		concatSlice = append(concatSlice, bs...)
	}

	return string(concatSlice)
}

const uintptrSize = unsafe.Sizeof(uintptr(0))

func uintptrToBytes(v uintptr) []byte {
	b := make([]byte, uintptrSize)

	switch uintptrSize {
	case 4:
		binary.LittleEndian.PutUint32(b, uint32(v))
	case 8:
		binary.LittleEndian.PutUint64(b, uint64(v))
	default:
		panic(fmt.Sprintf("unknown uintptr size: %v", uintptrSize))
	}

	return b
}
