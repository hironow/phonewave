package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

// ErrCircuitOpen is returned by Allow when the circuit breaker is open
// due to repeated delivery failures.
var ErrCircuitOpen = errors.New("circuit breaker open: delivery failures")

// circuitState represents the state of the circuit breaker.
type circuitState int

const (
	circuitClosed   circuitState = iota // normal operation
	circuitOpen                         // blocking calls
	circuitHalfOpen                     // probing
)

// defaultBackoffBase is the initial wait duration when reset time is unknown.
const defaultBackoffBase = 30 * time.Second

// defaultBackoffMax caps exponential backoff.
const defaultBackoffMax = 10 * time.Minute

// CircuitBreaker prevents cascading failures when delivery targets are
// temporarily unavailable. Delivery error classification is handled by the
// caller before calling RecordDeliveryError.
type CircuitBreaker struct {
	mu             sync.Mutex
	state          circuitState
	resetAt        time.Time
	backoffCurrent time.Duration
	logger         domain.Logger
	tripped        int
	lastTrip       time.Time
	lastReason     string
	notify         chan struct{}
}

// NewCircuitBreaker creates a circuit breaker in the closed state.
func NewCircuitBreaker(logger domain.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		state:          circuitClosed,
		backoffCurrent: defaultBackoffBase,
		logger:         logger,
		notify:         make(chan struct{}),
	}
}

// stateChanged closes the current notify channel and creates a new one.
// Must be called with mu held.
func (cb *CircuitBreaker) stateChanged() {
	close(cb.notify)
	cb.notify = make(chan struct{})
}

// Allow checks if a delivery is permitted. When the circuit is OPEN, it blocks
// until the backoff period elapses, then transitions to HALF_OPEN
// and returns nil. Returns context error if cancelled while waiting.
func (cb *CircuitBreaker) Allow(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		cb.mu.Lock()
		switch cb.state {
		case circuitClosed, circuitHalfOpen:
			cb.mu.Unlock()
			return nil
		case circuitOpen:
			if !cb.resetAt.IsZero() && time.Now().After(cb.resetAt) {
				cb.state = circuitHalfOpen
				cb.logger.Info("Circuit breaker: reset time reached, transitioning to HALF_OPEN (probe)")
				cb.mu.Unlock()
				return nil
			}
			if cb.resetAt.IsZero() && time.Since(cb.lastTrip) > cb.backoffCurrent {
				cb.state = circuitHalfOpen
				cb.logger.Info("Circuit breaker: backoff elapsed, transitioning to HALF_OPEN (probe)")
				cb.mu.Unlock()
				return nil
			}

			var waitDur time.Duration
			if !cb.resetAt.IsZero() {
				waitDur = time.Until(cb.resetAt)
				cb.logger.Warn("PAUSED — Delivery target unavailable. Resets at %s. Waiting...",
					cb.resetAt.Format("Jan 2, 3:04 PM (MST)"))
			} else {
				waitDur = cb.backoffCurrent - time.Since(cb.lastTrip)
				cb.logger.Warn("PAUSED — Delivery error. Waiting %v for recovery...", waitDur.Round(time.Second))
			}
			if waitDur <= 0 {
				waitDur = time.Second
			}
			notifyCh := cb.notify
			cb.mu.Unlock()

			timer := time.NewTimer(waitDur)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			case <-notifyCh:
				timer.Stop()
			}
		default:
			cb.mu.Unlock()
			return nil
		}
	}
}

// RecordDeliveryError updates the circuit breaker state based on a classified
// delivery error. Only transient errors trip the circuit breaker.
func (cb *CircuitBreaker) RecordDeliveryError(info domain.DeliveryErrorInfo) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !info.IsTrip() {
		return
	}

	cb.state = circuitOpen
	cb.tripped++
	cb.lastTrip = time.Now()
	cb.backoffCurrent *= 2
	if cb.backoffCurrent > defaultBackoffMax {
		cb.backoffCurrent = defaultBackoffMax
	}
	cb.resetAt = info.ResetAt
	cb.lastReason = deliveryPauseReason(info.Kind)
	cb.stateChanged()

	cb.logger.Warn("PAUSED — Circuit breaker OPEN. Delivery error detected, using backoff.")
}

// RecordSuccess resets the circuit breaker to closed state.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state != circuitClosed {
		cb.logger.Info("Circuit breaker: CLOSED (recovered)")
		cb.backoffCurrent = defaultBackoffBase
		cb.stateChanged()
	}
	cb.state = circuitClosed
	cb.resetAt = time.Time{}
	cb.lastReason = ""
}

// IsOpen returns true if the circuit breaker is in the open state.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == circuitOpen
}

// String returns a human-readable state description.
func (cb *CircuitBreaker) String() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case circuitClosed:
		return "CLOSED"
	case circuitOpen:
		if !cb.resetAt.IsZero() {
			return fmt.Sprintf("OPEN (resets at %s)", cb.resetAt.Format("15:04 MST"))
		}
		return "OPEN (backoff)"
	case circuitHalfOpen:
		return "HALF_OPEN"
	}
	return "UNKNOWN"
}

// Snapshot returns the current provider state snapshot matching the shared vocabulary.
func (cb *CircuitBreaker) Snapshot() domain.ProviderStateSnapshot {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return domain.ActiveProviderState()
	case circuitHalfOpen:
		return domain.ProviderStateSnapshot{
			State:           domain.ProviderStateDegraded,
			Reason:          domain.ProviderReasonProbe,
			RetryBudget:     1,
			ResumeCondition: domain.ResumeConditionProbeSucceeds,
		}
	case circuitOpen:
		snapshot := domain.ProviderStateSnapshot{
			State:       domain.ProviderStateWaiting,
			RetryBudget: 0,
		}
		if !cb.resetAt.IsZero() {
			snapshot.State = domain.ProviderStatePaused
			snapshot.Reason = cb.lastReason
			snapshot.ResumeAt = cb.resetAt
			snapshot.ResumeCondition = domain.ResumeConditionProviderReset
			return snapshot
		}
		snapshot.Reason = cb.lastReason
		snapshot.ResumeCondition = domain.ResumeConditionBackoffElapses
		return snapshot
	default:
		return domain.ActiveProviderState()
	}
}

func deliveryPauseReason(kind domain.DeliveryErrorKind) string {
	switch kind {
	case domain.DeliveryErrorTransient:
		return domain.ProviderReasonDeliveryRetryBackoff
	default:
		return "delivery_error"
	}
}
