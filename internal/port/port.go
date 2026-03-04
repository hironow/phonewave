package port

import (
	"context"
	"errors"

	"github.com/hironow/phonewave/internal/domain"
)

// ErrUnsupportedOS is returned by LocalNotifier on unsupported platforms.
var ErrUnsupportedOS = errors.New("notify: unsupported OS for local notifications")

// EventDispatcher processes events after persistence (e.g. POLICY dispatch).
type EventDispatcher interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

// Notifier sends a notification to the user.
type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}

// NopNotifier is a no-op notifier for tests and quiet mode.
type NopNotifier struct{}

func (NopNotifier) Notify(context.Context, string, string) error { return nil }

// PolicyMetrics records policy handler execution metrics.
type PolicyMetrics interface {
	RecordPolicyEvent(ctx context.Context, eventType string, status string)
}

// NopPolicyMetrics is a no-op metrics recorder for tests and quiet mode.
type NopPolicyMetrics struct{}

func (NopPolicyMetrics) RecordPolicyEvent(context.Context, string, string) {}
