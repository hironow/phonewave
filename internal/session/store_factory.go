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
