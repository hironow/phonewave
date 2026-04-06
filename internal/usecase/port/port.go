package port

import (
	"context"
	"errors"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

// ErrUnsupportedOS is returned by LocalNotifier on unsupported platforms.
var ErrUnsupportedOS = errors.New("notify: unsupported OS for local notifications")

// InitRunner handles init-time I/O: repo scanning, config writing, state dir creation.
type InitRunner interface {
	ScanAndInit(repoPaths []string, cfgPath string) (*domain.InitResult, error)
}

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
	// Append persists one or more events. Validation is performed before any writes.
	Append(events ...domain.Event) (domain.AppendResult, error)

	// LoadAll returns all events in chronological order.
	LoadAll() ([]domain.Event, domain.LoadResult, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]domain.Event, domain.LoadResult, error)

	// LoadAfterSeqNr returns all events with SeqNr > afterSeqNr,
	// ordered by SeqNr ascending. Used for snapshot-based recovery.
	LoadAfterSeqNr(afterSeqNr uint64) ([]domain.Event, domain.LoadResult, error)

	// LatestSeqNr returns the highest recorded SeqNr across all events.
	// Returns 0 if no events have a SeqNr assigned.
	LatestSeqNr() (uint64, error)
}

// SnapshotStore persists materialized projection state at a known SeqNr.
// Snapshots are an optimization — the system must function without them
// (falling back to full replay via LoadAll).
type SnapshotStore interface {
	// Save persists a snapshot. aggregateType identifies the projection kind.
	Save(ctx context.Context, aggregateType string, seqNr uint64, state []byte) error

	// Load returns the latest snapshot for the given aggregateType.
	// Returns (0, nil, nil) if no snapshot exists.
	Load(ctx context.Context, aggregateType string) (seqNr uint64, state []byte, err error)
}

// SeqAllocator assigns globally monotonic sequence numbers to events.
// Implemented by eventsource.SeqCounter (SQLite-backed).
type SeqAllocator interface {
	AllocSeqNr(ctx context.Context) (uint64, error)
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
	StageDelivery(ctx context.Context, dmailPath string, data []byte, targets []string) error
	FlushDeliveries(ctx context.Context) ([]domain.DeliveryFlushed, error)
	RecoverUnflushed() ([]domain.StagedDelivery, error)
	AllFlushedFor(dmailPath string) (bool, error)
	PruneFlushed(ctx context.Context) (int, error)
	Close() error
}

// DaemonEventEmitter wraps aggregate event production + persistence + dispatch.
// Implemented in usecase layer, injected into session via DaemonRunner.SetEmitter.
// Dispatch is best-effort: errors are logged but not returned.
type DaemonEventEmitter interface {
	EmitDelivery(sourcePath string, kind string, now time.Time) error
	EmitFailure(filePath string, kind string, errMsg string, now time.Time) error
	EmitScan(outboxDir string, delivered, errors int, now time.Time) error
	EmitRetry(name string, kind string, now time.Time) error
}

// InsightAppender writes insight entries to insight ledger files.
// Best-effort: errors should be logged but not propagated to callers.
type InsightAppender interface {
	Append(filename, kind, tool string, entry domain.InsightEntry) error
}

// NopInsightAppender is a no-op InsightAppender for tests and quiet mode.
type NopInsightAppender struct{}

func (NopInsightAppender) Append(string, string, string, domain.InsightEntry) error { return nil }

// InsightReader reads insight files for analysis.
type InsightReader interface {
	Read(filename string) (*domain.InsightFile, error)
}

// DaemonRunner represents a fully-constructed daemon ready for emitter injection.
// All infrastructure setup (config loading, store creation, lock acquisition) is done
// before the DaemonRunner is constructed. The usecase layer uses it only for:
// 1. Checking outbox count (business decision to not start if zero)
// 2. Injecting the DaemonEventEmitter (aggregate + store + dispatcher)
// 3. Running the daemon event loop
type DaemonRunner interface {
	// SetEmitter injects the event emitter (wraps aggregate + store + dispatcher).
	SetEmitter(e DaemonEventEmitter)
	// EventStore returns the session-layer event store for emitter construction.
	EventStore() EventStore
	// BuildNotifier returns the configured notifier for policy handlers.
	BuildNotifier() Notifier
	// BuildInsightAppender returns the configured InsightAppender for policy handlers.
	BuildInsightAppender() InsightAppender
	// BuildInsightReader returns the configured InsightReader for policy handlers.
	BuildInsightReader() InsightReader
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

func (NopDaemonRunner) SetEmitter(DaemonEventEmitter)         {}
func (NopDaemonRunner) EventStore() EventStore                { return nil }
func (NopDaemonRunner) BuildNotifier() Notifier               { return NopNotifier{} }
func (NopDaemonRunner) BuildInsightAppender() InsightAppender { return NopInsightAppender{} }
func (NopDaemonRunner) BuildInsightReader() InsightReader     { return nil }
func (NopDaemonRunner) RouteCount() int                       { return 0 }
func (NopDaemonRunner) OutboxCount() int                      { return 0 }
func (NopDaemonRunner) Run(context.Context) error             { return nil }
func (NopDaemonRunner) Close() error                          { return nil }

// DeliveryDedupStore provides exact dedup for D-Mail delivery.
// Prevents the same D-Mail content from being delivered twice using
// content-based idempotency keys backed by persistent storage.
type DeliveryDedupStore interface {
	HasDelivered(ctx context.Context, idempotencyKey string) (bool, error)
	RecordDelivery(ctx context.Context, idempotencyKey string, target string) error
	Close() error
}
