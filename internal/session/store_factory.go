package session

import (
	"path/filepath"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/eventsource"
)

// NewEventStore creates a FileEventStore at the conventional path.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func NewEventStore(stateDir string) phonewave.EventStore {
	return eventsource.NewFileEventStore(filepath.Join(stateDir, "events"))
}

// NewErrorStore creates a SQLiteErrorStore at {stateDir}/.run/errors.db.
// cmd layer should use this instead of instantiating directly.
func NewErrorStore(stateDir string) (*SQLiteErrorStore, error) {
	return NewSQLiteErrorStore(filepath.Join(stateDir, ".run"))
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
func ListExpiredEventFiles(stateDir string, days int) ([]string, error) {
	return eventsource.ListExpiredEventFiles(stateDir, days)
}

// PruneEventFiles deletes the named .jsonl files from the events directory.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func PruneEventFiles(stateDir string, files []string) ([]string, error) {
	return eventsource.PruneEventFiles(stateDir, files)
}
