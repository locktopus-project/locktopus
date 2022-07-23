package set

import (
	"testing"
)

func TestSet_Add(t *testing.T) {
	s := make(Set[int])

	if s.Has(0) {
		t.Error("Set should not have 0")
	}

	s.Add(0)

	if !s.Has(0) {
		t.Error("Set should have 0")
	}
}

func TestSet_Remove(t *testing.T) {
	s := make(Set[int])

	s.Add(0)

	if !s.Has(0) {
		t.Error("Set should have 0")
	}

	s.Remove(0)

	if s.Has(0) {
		t.Error("Set should not have 0")
	}
}

func TestSet_Clear(t *testing.T) {
	s := make(Set[int])

	s.Add(0)
	s.Add(1)

	if !s.Has(0) {
		t.Error("Set should have 0")
	}

	if !s.Has(1) {
		t.Error("Set should have 1")
	}

	s.Clear()

	if s.Has(0) {
		t.Error("Set should not have 0")
	}

	if s.Has(1) {
		t.Error("Set should not have 0")
	}
}
