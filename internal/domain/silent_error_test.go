package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestSilentError_ErrorMessage(t *testing.T) {
	// given
	err := &domain.SilentError{Err: fmt.Errorf("ecosystem has issues")}

	// then
	if err.Error() != "ecosystem has issues" {
		t.Errorf("Error() = %q, want %q", err.Error(), "ecosystem has issues")
	}
}

func TestSilentError_Unwrap(t *testing.T) {
	// given
	inner := fmt.Errorf("inner cause")
	err := &domain.SilentError{Err: inner}

	// then
	if !errors.Is(err, inner) {
		t.Error("errors.Is should find inner error through SilentError")
	}
}

func TestSilentError_DetectedByErrorsAs(t *testing.T) {
	// given
	err := fmt.Errorf("command: %w", &domain.SilentError{Err: fmt.Errorf("fail")})

	// then
	var se *domain.SilentError
	if !errors.As(err, &se) {
		t.Error("errors.As should find SilentError in chain")
	}
}
