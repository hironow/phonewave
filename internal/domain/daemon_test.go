package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestEffectiveIdleTimeout_Positive(t *testing.T) {
	// given: a positive idle timeout
	timeout := 30 * time.Minute

	// when
	effective := domain.EffectiveIdleTimeout(timeout)

	// then: returns the value as-is
	if effective != 30*time.Minute {
		t.Errorf("expected 30m, got %v", effective)
	}
}

func TestEffectiveIdleTimeout_Zero_SafetyCap(t *testing.T) {
	// given: zero idle timeout (safety cap)
	timeout := time.Duration(0)

	// when
	effective := domain.EffectiveIdleTimeout(timeout)

	// then: returns 24h safety cap
	if effective != 24*time.Hour {
		t.Errorf("expected 24h, got %v", effective)
	}
}

func TestEffectiveIdleTimeout_Negative_Disabled(t *testing.T) {
	// given: negative idle timeout (disabled)
	timeout := -1 * time.Second

	// when
	effective := domain.EffectiveIdleTimeout(timeout)

	// then: returns 0 (disabled)
	if effective != 0 {
		t.Errorf("expected 0 (disabled), got %v", effective)
	}
}

func TestDefaultIdleTimeout_Is30Minutes(t *testing.T) {
	if domain.DefaultIdleTimeout != 30*time.Minute {
		t.Errorf("expected 30m, got %v", domain.DefaultIdleTimeout)
	}
}

func TestRunDaemonCommand_IdleTimeout(t *testing.T) {
	// given
	ri, _ := domain.NewRetryInterval(60 * time.Second)
	mr, _ := domain.NewMaxRetries(10)

	// when
	cmd := domain.NewRunDaemonCommand(false, false, ri, mr, 45*time.Minute)

	// then
	if cmd.IdleTimeout() != 45*time.Minute {
		t.Errorf("expected 45m, got %v", cmd.IdleTimeout())
	}
}
