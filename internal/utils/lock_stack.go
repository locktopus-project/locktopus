package internal

import (
	"encoding/binary"
	"fmt"
	"sync"
	"unsafe"

	chainMutex "github.com/xshkut/distributed-lock/pgk/chain_mutex"
	setCounter "github.com/xshkut/distributed-lock/pgk/set_counter"
)

type LockResource struct {
	lockType     chainMutex.LockType
	resourcePath []string
}

func NewLockResource(lockType chainMutex.LockType, resourcePath []string) LockResource {
	return LockResource{
		lockType:     lockType,
		resourcePath: resourcePath,
	}
}

type LQLayer struct {
	id        int64
	resources []LockResource
}

type LockQueue struct {
	mx           sync.Mutex
	nextLayerID  int64
	layers       []LQLayer
	tokens       setCounter.SetCounter
	chainMutexes map[string]*chainMutex.Vertice
}

func NewLockQueue() *LockQueue {
	return &LockQueue{
		nextLayerID:  1,
		layers:       []LQLayer{},
		tokens:       setCounter.NewSetCounter(),
		chainMutexes: make(map[string]*chainMutex.Vertice),
	}
}

// Lock locks the whole layer. Returns when the layer is locked. Use 'release' to unlock or cancel the lock (even if the lock has not been returned yet).
func (lq *LockQueue) LockLayer(resources []LockResource, unlock <-chan interface{}) {
	lq.mx.Lock()

	lq.layers = append(lq.layers, LQLayer{
		id:        lq.nextLayerID,
		resources: resources,
	})

	lq.nextLayerID++

	lq.mx.Unlock()
}

func (lq *LockQueue) getLockResourceChainMutex(lqr LockResource) {
	// lq.mx.Lock()
	// defer lq.mx.Unlock()

	// path := lq.getResourceCompactPath(lqr.resourcePath)
	// defer lq.releaseResourcePath(lqr.resourcePath)

	// // if cm, ok := lq.chainMutexes[path]; ok {

	// // }

}

func (lq *LockQueue) getResourceCompactPath(path []string) string {
	tokenPointers := make([]uintptr, len(path))

	for i, segment := range path {
		p := lq.tokens.Store(segment)

		tokenPointers[i] = uintptr(unsafe.Pointer(p))
	}

	return buildCompactPathFromPointers(tokenPointers)
}

func (lq *LockQueue) releaseResourcePath(path []string) {
	for _, segment := range path {
		lq.tokens.Release(segment)
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
