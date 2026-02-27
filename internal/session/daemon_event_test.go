package session_test

import (
	"encoding/json"
	"testing"
	"time"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/eventsource"
)

func TestDaemon_EmitsDeliverySucceededEvent(t *testing.T) {
	// given
	eventsDir := t.TempDir()
	store := eventsource.NewFileEventStore(eventsDir)

	ev, err := phonewave.NewEvent(phonewave.EventDeliverySucceeded, phonewave.DeliverySucceededData{
		Kind:        "report",
		SourcePath:  "outbox/rp-001.md",
		DeliveredTo: []string{"inbox/rp-001.md"},
	}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	if err := store.Append(ev); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// then
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != phonewave.EventDeliverySucceeded {
		t.Errorf("expected %s, got %s", phonewave.EventDeliverySucceeded, events[0].Type)
	}
	var data phonewave.DeliverySucceededData
	if err := json.Unmarshal(events[0].Data, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Kind != "report" {
		t.Errorf("expected kind 'report', got %q", data.Kind)
	}
}

func TestDaemon_EmitsDeliveryFailedEvent(t *testing.T) {
	// given
	eventsDir := t.TempDir()
	store := eventsource.NewFileEventStore(eventsDir)

	ev, err := phonewave.NewEvent(phonewave.EventDeliveryFailed, phonewave.DeliveryFailedData{
		Kind:       "feedback",
		SourcePath: "outbox/fb-001.md",
		Reason:     "no matching route",
		Attempt:    1,
	}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	if err := store.Append(ev); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// then
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != phonewave.EventDeliveryFailed {
		t.Errorf("expected %s, got %s", phonewave.EventDeliveryFailed, events[0].Type)
	}
}
