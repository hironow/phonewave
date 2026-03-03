package eventsource_test

import (
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/eventsource"
)

func TestFileEventStore_AppendAndLoadAll(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	ev, err := domain.NewEvent(domain.EventDeliveryCompleted, map[string]string{"to": "inbox"}, time.Now())
	if err != nil {
		t.Fatalf("new event: %v", err)
	}

	// when
	if err := store.Append(ev); err != nil {
		t.Fatalf("append: %v", err)
	}
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}

	// then
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != ev.ID {
		t.Errorf("expected ID %s, got %s", ev.ID, events[0].ID)
	}
	if events[0].Type != domain.EventDeliveryCompleted {
		t.Errorf("expected type %s, got %s", domain.EventDeliveryCompleted, events[0].Type)
	}
}

func TestFileEventStore_LoadSince_FiltersOlderEvents(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	old, err := domain.NewEvent(domain.EventScanCompleted, nil, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	recent, err := domain.NewEvent(domain.EventDeliveryCompleted, nil, time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	if err := store.Append(old, recent); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	events, err := store.LoadSince(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load since: %v", err)
	}

	// then
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != recent.ID {
		t.Errorf("expected recent event, got %s", events[0].ID)
	}
}

func TestFileEventStore_AppendRejectsInvalidEvent(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	invalid := domain.Event{} // missing ID, Type, Timestamp

	// when
	err := store.Append(invalid)

	// then
	if err == nil {
		t.Fatal("expected error for invalid event, got nil")
	}
}

func TestFileEventStore_LoadAll_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	// when
	events, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestFileEventStore_LoadAll_NonexistentDir(t *testing.T) {
	// given
	store := eventsource.NewFileEventStore("/nonexistent/path/events")

	// when
	events, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}
}

func TestFileEventStore_ImplementsInterface(t *testing.T) {
	// Compile-time check is in store_file.go, but verify at runtime too.
	var _ domain.EventStore = eventsource.NewFileEventStore("")
}
