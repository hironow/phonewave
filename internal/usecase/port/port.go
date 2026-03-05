package port

import (
	"context"
	"errors"
	"time"

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

// EventStore is the interface for an append-only event log.
type EventStore interface {
	Append(events ...domain.Event) error
	LoadAll() ([]domain.Event, error)
	LoadSince(after time.Time) ([]domain.Event, error)
}

// ErrorQueueStore manages failed D-Mail delivery records with atomic claim
// semantics to prevent duplicate processing across concurrent daemon instances.
type ErrorQueueStore interface {
	Enqueue(name string, data []byte, meta domain.ErrorMetadata) error
	ClaimPendingRetries(claimerID string, maxRetries int) ([]domain.ErrorEntry, error)
	PendingCount(maxRetries int) (int, error)
	IncrementRetry(name string, newError string) error
	MarkResolved(name string) error
	Close() error
}

// DeliveryStore manages staged delivery intents with transactional guarantees.
// Stage records the intent; Flush writes files and marks them done.
type DeliveryStore interface {
	StageDelivery(dmailPath string, data []byte, targets []string) error
	FlushDeliveries() ([]domain.DeliveryFlushed, error)
	RecoverUnflushed() ([]domain.StagedDelivery, error)
	AllFlushedFor(dmailPath string) (bool, error)
	PruneFlushed() (int, error)
	Close() error
}

// DaemonRunner represents a fully-constructed daemon ready for policy engine injection.
// All infrastructure setup (config loading, store creation, lock acquisition) is done
// before the DaemonRunner is constructed. The usecase layer uses it only for:
// 1. Checking outbox count (business decision to not start if zero)
// 2. Injecting the PolicyEngine dispatcher
// 3. Running the daemon event loop
type DaemonRunner interface {
	// SetDispatcher injects the event dispatcher (PolicyEngine) into the daemon.
	SetDispatcher(d EventDispatcher)
	// BuildNotifier returns the configured notifier for policy handlers.
	BuildNotifier() Notifier
	// RouteCount returns the number of resolved delivery routes.
	RouteCount() int
	// OutboxCount returns the number of watched outbox directories.
	OutboxCount() int
	// Run starts the daemon event loop. Blocks until ctx is cancelled.
	Run(ctx context.Context) error
	// Close releases all resources (stores, logs, locks).
	Close() error
}

// NopDaemonRunner is a no-op DaemonRunner for tests.
type NopDaemonRunner struct{}

func (NopDaemonRunner) SetDispatcher(EventDispatcher)       {}
func (NopDaemonRunner) BuildNotifier() Notifier             { return NopNotifier{} }
func (NopDaemonRunner) RouteCount() int                     { return 0 }
func (NopDaemonRunner) OutboxCount() int                    { return 0 }
func (NopDaemonRunner) Run(context.Context) error           { return nil }
func (NopDaemonRunner) Close() error                        { return nil }
