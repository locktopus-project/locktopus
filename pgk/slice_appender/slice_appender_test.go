package sliceappender

import (
	"reflect"
	"testing"
)

func TestSliceAppender_NewSliceAppender(t *testing.T) {
	s := NewSliceAppender[int]()

	if !reflect.DeepEqual(s.Value(), []int{}) {
		t.Error("NewSliceAppender should return empty slice")
	}
}

func TestSliceAppender_Append(t *testing.T) {
	s := NewSliceAppender[int]()

	s.Append(1, 2, 3)

	if !reflect.DeepEqual(s.Value(), []int{1, 2, 3}) {
		t.Error("Append did not append values")
	}

	s.Append(4, 5, 6)

	if !reflect.DeepEqual(s.Value(), []int{1, 2, 3, 4, 5, 6}) {
		t.Error("Append did not append values")
	}
}
