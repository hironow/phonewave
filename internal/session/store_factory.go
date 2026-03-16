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
