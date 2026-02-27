package eventsource_test

import (
	"testing"
	"time"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/eventsource"
)

func TestFileEventStore_AppendAndLoadAll(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	ts := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	ev1, err := phonewave.NewEvent(phonewave.EventDeliverySucceeded, phonewave.DeliverySucceededData{
		Kind:        "report",
		SourcePath:  "outbox/rp-001.md",
		DeliveredTo: []string{"inbox/rp-001.md"},
	}, ts)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	ev2, err := phonewave.NewEvent(phonewave.EventDeliveryFailed, phonewave.DeliveryFailedData{
		Kind:       "feedback",
		SourcePath: "outbox/fb-001.md",
		Reason:     "no route",
		Attempt:    1,
	}, ts.Add(time.Second))
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	if err := store.Append(ev1); err != nil {
		t.Fatalf("Append ev1: %v", err)
	}
	if err := store.Append(ev2); err != nil {
		t.Fatalf("Append ev2: %v", err)
	}

	// then
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != phonewave.EventDeliverySucceeded {
		t.Errorf("expected first event type %s, got %s", phonewave.EventDeliverySucceeded, events[0].Type)
	}
	if events[1].Type != phonewave.EventDeliveryFailed {
		t.Errorf("expected second event type %s, got %s", phonewave.EventDeliveryFailed, events[1].Type)
	}
}

func TestFileEventStore_LoadSince(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	base := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)

	for i := range 3 {
		ev, _ := phonewave.NewEvent(phonewave.EventDeliverySucceeded, phonewave.DeliverySucceededData{Kind: "report"}, base.Add(time.Duration(i)*time.Hour))
		if err := store.Append(ev); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	// when: load events after the first one
	cutoff := base.Add(30 * time.Minute)
	events, err := store.LoadSince(cutoff)

	// then
	if err != nil {
		t.Fatalf("LoadSince: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events after cutoff, got %d", len(events))
	}
}

func TestFileEventStore_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	// when
	events, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("LoadAll on empty: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestFileEventStore_DailyRotation(t *testing.T) {
	// given: events across 2 days
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	day1 := time.Date(2026, 2, 27, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	ev1, _ := phonewave.NewEvent(phonewave.EventDeliverySucceeded, phonewave.DeliverySucceededData{Kind: "report"}, day1)
	ev2, _ := phonewave.NewEvent(phonewave.EventDeliveryFailed, phonewave.DeliveryFailedData{Kind: "feedback", Reason: "err"}, day2)

	// when
	if err := store.Append(ev1, ev2); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// then: both events loadable and in chronological order
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if !events[0].Timestamp.Before(events[1].Timestamp) {
		t.Error("expected chronological order")
	}
}

func TestFileEventStore_RejectInvalidEvent(t *testing.T) {
	// given: event with empty ID
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	bad := phonewave.Event{Type: "test", Timestamp: time.Now()}

	// when
	err := store.Append(bad)

	// then
	if err == nil {
		t.Fatal("expected validation error")
	}
}
