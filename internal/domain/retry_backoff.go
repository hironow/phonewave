package domain

import (
	"math/rand/v2"
	"time"
)

// RetryBackoff implements exponential backoff with jitter for retry burst control.
// When consecutive retries fail, the interval increases exponentially up to a max.
// A successful retry resets the interval to the base value.
type RetryBackoff struct {
	base    time.Duration
	max     time.Duration
	current time.Duration
}

// NewRetryBackoff creates a new RetryBackoff with the given base and max intervals.
func NewRetryBackoff(base, max time.Duration) *RetryBackoff {
	return &RetryBackoff{base: base, max: max, current: base}
}

// Next returns the current interval with ±25% jitter applied.
func (b *RetryBackoff) Next() time.Duration {
	// jitter: ±25% of current
	quarter := b.current / 4
	jitter := time.Duration(rand.Int64N(int64(quarter)*2+1)) - quarter
	return b.current + jitter
}

// RecordSuccess resets the backoff interval to the base value.
func (b *RetryBackoff) RecordSuccess() {
	b.current = b.base
}

// RecordFailure doubles the backoff interval, capped at the max value.
func (b *RetryBackoff) RecordFailure() {
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}
}
