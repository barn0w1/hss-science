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

func TestSentinelErrors_WrappedMatch(t *testing.T) {
	wrapped := fmt.Errorf("user 123: %w", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("expected errors.Is(wrapped, ErrNotFound) to be true")
	}
	if errors.Is(wrapped, ErrAlreadyExists) {
		t.Error("expected errors.Is(wrapped, ErrAlreadyExists) to be false")
	}
}

func TestSentinelErrors_NilNotMatch(t *testing.T) {
	if errors.Is(nil, ErrNotFound) {
		t.Error("expected errors.Is(nil, ErrNotFound) to be false")
	}
}
