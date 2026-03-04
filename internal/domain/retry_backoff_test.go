package domain

import (
	"testing"
	"time"
)

func TestRetryBackoff_InitialInterval(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 60*time.Second)

	// when
	d := b.Next()

	// then: should be within ±25% of base (1s)
	if d < 750*time.Millisecond || d > 1250*time.Millisecond {
		t.Errorf("initial interval: got %v, want ~1s (±25%%)", d)
	}
}

func TestRetryBackoff_ExponentialIncrease(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 60*time.Second)

	// when: record 3 consecutive failures
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()

	// then: base should be 8s (1s * 2^3) — Next with jitter should be ~6-10s
	d := b.Next()
	if d < 6*time.Second || d > 10*time.Second {
		t.Errorf("after 3 failures: got %v, want ~8s (±25%%)", d)
	}
}

func TestRetryBackoff_CappedAtMax(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 10*time.Second)

	// when: record many failures (should cap at max)
	for range 20 {
		b.RecordFailure()
	}

	// then: should be within ±25% of max (10s), never exceed 12.5s
	d := b.Next()
	if d > 12500*time.Millisecond {
		t.Errorf("capped interval: got %v, should not exceed 12.5s (max 10s + 25%% jitter)", d)
	}
	if d < 7500*time.Millisecond {
		t.Errorf("capped interval: got %v, should be at least 7.5s (max 10s - 25%% jitter)", d)
	}
}

func TestRetryBackoff_ResetOnSuccess(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 60*time.Second)
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()

	// when: record success
	b.RecordSuccess()

	// then: should be back to base interval (~1s)
	d := b.Next()
	if d < 750*time.Millisecond || d > 1250*time.Millisecond {
		t.Errorf("after reset: got %v, want ~1s (±25%%)", d)
	}
}

func TestRetryBackoff_ConsecutiveFailures(t *testing.T) {
	// given
	b := NewRetryBackoff(100*time.Millisecond, 10*time.Second)

	// when/then: each failure should roughly double the interval
	b.RecordFailure() // current = 200ms
	d1 := b.Next()

	b.RecordFailure() // current = 400ms
	d2 := b.Next()

	// d2 should be roughly 2x d1 (within jitter bounds)
	if d2 < d1 {
		t.Errorf("second failure interval %v should be > first %v", d2, d1)
	}
}
