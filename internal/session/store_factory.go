package session

import (
	"path/filepath"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/eventsource"
)

// EventsDir returns the path to the events directory within a state directory.
func EventsDir(stateDir string) string {
	return filepath.Join(stateDir, "events")
}

// NewEventStore creates an EventStore backed by daily JSONL files in the events directory.
func NewEventStore(stateDir string) phonewave.EventStore {
	return eventsource.NewFileEventStore(EventsDir(stateDir))
}

// NewOutboxStore creates an OutboxStore backed by SQLite at stateDir/.run/outbox.db.
// The delivery function is invoked for each item during Flush.
func NewOutboxStore(stateDir string, deliverFn DeliverFunc) (*SQLiteOutboxStore, error) {
	dbPath := filepath.Join(stateDir, ".run", "outbox.db")
	return NewSQLiteOutboxStore(dbPath, deliverFn)
}
