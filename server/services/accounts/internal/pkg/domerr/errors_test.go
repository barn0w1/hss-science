package domerr

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{ErrNotFound, ErrAlreadyExists, ErrUnauthorized, ErrInternal}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %v should not match %v", a, b)
			}
		}
	}
}

func TestIs_WrappedError(t *testing.T) {
	wrapped := fmt.Errorf("user 123: %w", ErrNotFound)
	if !Is(wrapped, ErrNotFound) {
		t.Error("expected Is(wrapped, ErrNotFound) to be true")
	}
	if Is(wrapped, ErrAlreadyExists) {
		t.Error("expected Is(wrapped, ErrAlreadyExists) to be false")
	}
}

func TestIs_NilError(t *testing.T) {
	if Is(nil, ErrNotFound) {
		t.Error("expected Is(nil, ErrNotFound) to be false")
	}
}
