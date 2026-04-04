package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/eventsource"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// NewEventStore creates a FileEventStore at the conventional path.
// eventsource is the event persistence adapter (AWS Event Sourcing pattern).
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func NewEventStore(stateDir string, logger domain.Logger) port.EventStore {
	raw := eventsource.NewFileEventStore(filepath.Join(stateDir, "events"), logger)
	return NewSpanEventStore(raw)
}

// EnsureRunDir creates the .run/ directory under stateDir if it does not exist.
// Call once before opening stores that write to .run/ (idempotent).
func EnsureRunDir(stateDir string) error {
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("ensure run dir: %w", err)
	}
	return nil
}

// NewErrorQueueStore creates a SQLiteErrorQueueStore at {stateDir}/.run/error_queue.db.
// cmd layer should use this instead of instantiating directly.
func NewErrorQueueStore(stateDir string) (*SQLiteErrorQueueStore, error) {
	return NewSQLiteErrorQueueStore(stateDir)
}

// NewDeliveryStore creates a SQLiteDeliveryStore at {stateDir}/.run/delivery.db.
// cmd layer should use this instead of instantiating directly (ADR S0008).
func NewDeliveryStore(stateDir string) (*SQLiteDeliveryStore, error) {
	return NewSQLiteDeliveryStore(stateDir)
}

// EnsureCutover performs the one-time event store cutover (ADR S0040) and
// returns a SeqAllocator for global SeqNr assignment. The caller must call
// the returned close function when done. Idempotent — no-op if already done.
func EnsureCutover(ctx context.Context, stateDir, aggregateType string, logger domain.Logger) (port.SeqAllocator, func(), error) {
	if err := EnsureRunDir(stateDir); err != nil {
		return nil, nil, err
	}
	// seq.db lives at stateDir root (NOT .run/) — .run/ is ephemeral and may
	// be deleted by rebuild or doctor --repair. seq.db must survive those operations.
	seqCounter, err := eventsource.NewSeqCounter(filepath.Join(stateDir, "seq.db"))
	if err != nil {
		return nil, nil, fmt.Errorf("ensure cutover: seq counter: %w", err)
	}
	snapshotStore := eventsource.NewFileSnapshotStore(filepath.Join(stateDir, "snapshots"))
	rawStore := eventsource.NewFileEventStore(filepath.Join(stateDir, "events"), logger)

	result, err := eventsource.RunCutover(ctx, rawStore, snapshotStore, seqCounter, aggregateType, logger)
	if err != nil {
		seqCounter.Close()
		return nil, nil, fmt.Errorf("ensure cutover: %w", err)
	}
	if !result.AlreadyDone {
		logger.Info("event store cutover complete: %d pre-cutover events, SeqNr=%d", result.EventCount, result.CutoverSeqNr)
	}
	closer := func() { seqCounter.Close() }
	return seqCounter, closer, nil
}

// ListExpiredEventFiles returns .jsonl files older than the given days.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func ListExpiredEventFiles(ctx context.Context, stateDir string, days int) ([]string, error) {
	_, span := platform.Tracer.Start(ctx, "eventsource.list_expired")
	defer span.End()

	files, err := eventsource.ListExpiredEventFiles(stateDir, days)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.list_expired"))
	}
	span.SetAttributes(attribute.Int("event.count.out", len(files)))
	return files, err
}

// PruneEventFiles deletes the named .jsonl files from the events directory.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func PruneEventFiles(ctx context.Context, stateDir string, files []string) ([]string, error) {
	_, span := platform.Tracer.Start(ctx, "eventsource.prune")
	defer span.End()

	span.SetAttributes(attribute.Int("event.count.in", len(files)))
	deleted, err := eventsource.PruneEventFiles(stateDir, files)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "eventsource.prune"))
	}
	span.SetAttributes(attribute.Int("event.count.out", len(deleted)))
	return deleted, err
}
