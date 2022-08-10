package lockqueue

import (
	"encoding/binary"
	"sort"
	"strings"
	"unsafe"
)

type tokenRef uintptr

const uintptrSize = int(unsafe.Sizeof(uintptr(0)))

func concatSegmentRefs(pointers []tokenRef) string {
	concatSlice := make([]byte, len(pointers)*uintptrSize)

	for i, p := range pointers {
		binary.PutVarint(concatSlice[i*uintptrSize:], int64(p))
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
