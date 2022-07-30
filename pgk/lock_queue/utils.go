package lockqueue

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"unsafe"
)

type tokenRef uintptr

const uintptrSize = unsafe.Sizeof(uintptr(0))

func uintptrToBytes(v tokenRef) []byte {
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

func concatSegmentRefs(pointers []tokenRef) string {
	concatSlice := make([]byte, 0)

	for _, p := range pointers {
		bs := uintptrToBytes(p)
		concatSlice = append(concatSlice, bs...)
	}

	return string(concatSlice)
}

func truncateAfterPrefixes(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}

	output := []string{}

	sort.Strings(input)

	for _, str := range input {
		if len(output) == 0 {
			output = append(output, str)
			continue
		}

		if strings.Contains(str, output[len(output)-1]) {
			continue
		}

		output = append(output, str)
	}

	return output
}
