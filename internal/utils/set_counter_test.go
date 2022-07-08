package internal

import (
	"testing"
)

func TestTokenSet_Get(t *testing.T) {
	tm := NewSetCounter()

	if got := tm.Get("some_key"); got != nil {
		t.Errorf("TokenSet.Get() = %v, want nil", got)
	}

	t.Run("Getting non-existing key should nil", func(t *testing.T) {
		if got := tm.Get("some_key"); got != nil {
			t.Errorf("TokenSet.Get() = %v, want nil", got)
		}
	})

	t.Run("Setting first-set key should 1", func(t *testing.T) {
		if got := tm.Store("some_key"); *got != 1 {
			t.Errorf("TokenSet.Get() = %v, want 1", got)
		}
	})

	t.Run("Getting existing key should return its value", func(t *testing.T) {
		if got := tm.Get("some_key"); *got != 1 {
			t.Errorf("TokenSet.Get() = %v, want 1", got)
		}
	})

	t.Run("Setting existing key should increment its value", func(t *testing.T) {
		if got := tm.Store("some_key"); *got != 2 {
			t.Errorf("TokenSet.Get() = %v, want 2", got)
		}
	})

	t.Run("Releasing existing key should decrement its value", func(t *testing.T) {
		if got := tm.Release("some_key"); *got != 1 {
			t.Errorf("TokenSet.Get() = %v, want 2", got)
		}
		if got := tm.Release("some_key"); got != nil {
			t.Errorf("TokenSet.Get() = %v, want nil", got)
		}
	})

	t.Run("Releasing non-existing key should panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()

		tm.Release("some_key")
	})
}
