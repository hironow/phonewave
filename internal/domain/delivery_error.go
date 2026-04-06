package domain

import "time"

// DeliveryErrorKind classifies delivery failures for circuit breaker decisions.
type DeliveryErrorKind int

const (
	// DeliveryErrorNone indicates no error.
	DeliveryErrorNone DeliveryErrorKind = iota
	// DeliveryErrorTransient indicates a temporary failure (filesystem busy, permission denied).
	// Transient errors trip the circuit breaker.
	DeliveryErrorTransient
	// DeliveryErrorPersistent indicates a permanent failure (target not found, malformed D-Mail).
	// Persistent errors go to the error queue without tripping the circuit breaker.
	DeliveryErrorPersistent
)

// DeliveryErrorInfo carries classified error info for the delivery circuit breaker.
type DeliveryErrorInfo struct {
	Kind    DeliveryErrorKind
	ResetAt time.Time // zero = unknown
	Err     error
}

// IsTrip returns true if this error should trip the circuit breaker.
// Only transient errors trip; persistent errors are routed to the error queue.
func (i DeliveryErrorInfo) IsTrip() bool {
	return i.Kind == DeliveryErrorTransient
}
