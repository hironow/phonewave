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
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	ev, err := domain.NewEvent(domain.EventDeliveryCompleted, map[string]string{"to": "inbox"}, time.Now())
	if err != nil {
		t.Fatalf("new event: %v", err)
	}

	// when
	result, err := store.Append(ev)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	events, loadResult, err := store.LoadAll()
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
	if result.BytesWritten <= 0 {
		t.Errorf("expected positive bytes written, got %d", result.BytesWritten)
	}
	if loadResult.FileCount != 1 {
		t.Errorf("expected 1 file, got %d", loadResult.FileCount)
	}
	if loadResult.CorruptLineCount != 0 {
		t.Errorf("expected 0 corrupt lines, got %d", loadResult.CorruptLineCount)
	}
}

func TestFileEventStore_LoadSince_FiltersOlderEvents(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	old, err := domain.NewEvent(domain.EventScanCompleted, nil, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	recent, err := domain.NewEvent(domain.EventDeliveryCompleted, nil, time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	if _, err := store.Append(old, recent); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	events, loadResult, err := store.LoadSince(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
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
	if loadResult.FileCount != 2 {
		t.Errorf("expected 2 files (2 dates), got %d", loadResult.FileCount)
	}
}

func TestFileEventStore_AppendRejectsInvalidEvent(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	invalid := domain.Event{} // missing ID, Type, Timestamp

	// when
	_, err := store.Append(invalid)

	// then
	if err == nil {
		t.Fatal("expected error for invalid event, got nil")
	}
}

func TestFileEventStore_LoadAll_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

	// when
	events, _, err := store.LoadAll()

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
	store := eventsource.NewFileEventStore("/nonexistent/path/events", &domain.NopLogger{})

	// when
	events, _, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}
}

func TestFileEventStore_ImplementsInterface(t *testing.T) {
	// Duck typing: FileEventStore satisfies port.EventStore via Go structural typing.
	// Verified by store_factory.go which assigns *FileEventStore to port.EventStore.
	store := eventsource.NewFileEventStore(t.TempDir(), &domain.NopLogger{})
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestFileEventStore_LoadAfterSeqNr_FiltersAndSorts(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	now := time.Now()
	ev1, _ := domain.NewEvent(domain.EventDeliveryCompleted, nil, now)
	ev1.SeqNr = 1
	ev2, _ := domain.NewEvent(domain.EventScanCompleted, nil, now.Add(time.Second))
	ev2.SeqNr = 2
	ev3, _ := domain.NewEvent(domain.EventDeliveryCompleted, nil, now.Add(2*time.Second))
	ev3.SeqNr = 3
	if _, err := store.Append(ev1, ev2, ev3); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when — load events after SeqNr 1
	events, _, err := store.LoadAfterSeqNr(1)
	if err != nil {
		t.Fatalf("load after seq nr: %v", err)
	}

	// then — should return ev2 and ev3, sorted by SeqNr ascending
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].SeqNr != 2 {
		t.Errorf("expected SeqNr 2, got %d", events[0].SeqNr)
	}
	if events[1].SeqNr != 3 {
		t.Errorf("expected SeqNr 3, got %d", events[1].SeqNr)
	}
}

func TestFileEventStore_LoadAfterSeqNr_ReturnsEmptyForHighSeqNr(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	ev, _ := domain.NewEvent(domain.EventDeliveryCompleted, nil, time.Now())
	ev.SeqNr = 5
	if _, err := store.Append(ev); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	events, _, err := store.LoadAfterSeqNr(100)
	if err != nil {
		t.Fatalf("load after seq nr: %v", err)
	}

	// then
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestFileEventStore_LoadAfterSeqNr_SkipsZeroSeqNr(t *testing.T) {
	// given — pre-cutover events have SeqNr=0
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	legacy, _ := domain.NewEvent(domain.EventDeliveryCompleted, nil, time.Now())
	// SeqNr defaults to 0 (pre-cutover)
	postCutover, _ := domain.NewEvent(domain.EventScanCompleted, nil, time.Now().Add(time.Second))
	postCutover.SeqNr = 1
	if _, err := store.Append(legacy, postCutover); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when — LoadAfterSeqNr(0) returns all post-cutover events
	events, _, err := store.LoadAfterSeqNr(0)
	if err != nil {
		t.Fatalf("load after seq nr: %v", err)
	}

	// then — only the post-cutover event (SeqNr > 0)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].SeqNr != 1 {
		t.Errorf("expected SeqNr 1, got %d", events[0].SeqNr)
	}
}

func TestFileEventStore_LatestSeqNr(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	now := time.Now()
	ev1, _ := domain.NewEvent(domain.EventDeliveryCompleted, nil, now)
	ev1.SeqNr = 3
	ev2, _ := domain.NewEvent(domain.EventScanCompleted, nil, now.Add(time.Second))
	ev2.SeqNr = 7
	ev3, _ := domain.NewEvent(domain.EventDeliveryCompleted, nil, now.Add(2*time.Second))
	ev3.SeqNr = 5
	if _, err := store.Append(ev1, ev2, ev3); err != nil {
		t.Fatalf("append: %v", err)
	}

	// when
	seqNr, err := store.LatestSeqNr()
	if err != nil {
		t.Fatalf("latest seq nr: %v", err)
	}

	// then
	if seqNr != 7 {
		t.Errorf("expected latest SeqNr 7, got %d", seqNr)
	}
}

func TestFileEventStore_LatestSeqNr_EmptyStore(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

	// when
	seqNr, err := store.LatestSeqNr()
	if err != nil {
		t.Fatalf("latest seq nr: %v", err)
	}

	// then
	if seqNr != 0 {
		t.Errorf("expected SeqNr 0 for empty store, got %d", seqNr)
	}
}
