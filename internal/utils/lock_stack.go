package internal

import (
	"encoding/binary"
	"fmt"
	"sync"
	"unsafe"
)

type LQResource struct {
	lockType     LockType
	resourcePath []string
}

func NewLockResource(lockType LockType, resourcePath []string) LQResource {
	return LQResource{
		lockType:     lockType,
		resourcePath: resourcePath,
	}
}

type LQLayer struct {
	id        int64
	resources []LQResource
}

type LockQueue struct {
	mx             sync.Mutex
	nextLayerID    int64
	layers         []LQLayer
	resourcesPaths SetCounter
	tokens         SetCounter
}

func NewLockQueue() *LockQueue {
	return &LockQueue{
		nextLayerID:    1,
		layers:         []LQLayer{},
		resourcesPaths: NewSetCounter(),
		tokens:         NewSetCounter(),
	}
}

// Lock locks the whole layer. Returns when the layer is locked. Use 'release' to unlock or cancel the lock (even if the lock has not been returned yet).
func (lq *LockQueue) LockLayer(resources []LQResource, unlock <-chan interface{}) {
	lq.mx.Lock()

	lq.layers = append(lq.layers, LQLayer{
		id:        lq.nextLayerID,
		resources: resources,
	})

	lq.nextLayerID++

	lq.mx.Unlock()

}

func (lq *LockQueue) lockResource(lqr LQResource, release <-chan interface{}) {
	lq.mx.Lock()

	tokenPointers := make([]uintptr, len(lq.layers))
	for _, r := range lqr.resourcePath {
		p := lq.resourcesPaths.Store(r)
		tokenPointers = append(tokenPointers, uintptr(unsafe.Pointer(p)))

	}

	// compactedPath := buildCOmpactedPath(tokenPointers)

	lq.mx.Unlock()
}

func buildCOmpactedPath(pointers []uintptr) string {
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
