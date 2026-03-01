package session_test

import (
	"fmt"
	"io"
	"testing"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/session"
)

func newTestDaemonSession(t *testing.T) (*session.DaemonSession, string) {
	t.Helper()
	dir := t.TempDir()
	errorQueue := testErrorQueueStore(t)
	eventStore := session.NewEventStore(dir)
	dlog, err := session.NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("create delivery log: %v", err)
	}
	t.Cleanup(func() { dlog.Close() })
	logger := phonewave.NewLogger(io.Discard, false)
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: "/tmp/outbox", ToInboxes: []string{"/tmp/inbox"}},
	}
	ds := session.NewDaemonSession(errorQueue, eventStore, dlog, routes, dir, logger)
	return ds, dir
}

func TestNewDaemonSession(t *testing.T) {
	// given/when
	ds, _ := newTestDaemonSession(t)

	// then
	if ds == nil {
		t.Fatal("DaemonSession should not be nil")
	}
	if ds.ErrorQueue == nil {
		t.Error("ErrorQueue should be set")
	}
	if ds.EventStore == nil {
		t.Error("EventStore should be set")
	}
	if ds.DeliveryLog == nil {
		t.Error("DeliveryLog should be set")
	}
	if len(ds.Routes) != 1 {
		t.Errorf("Routes: got %d, want 1", len(ds.Routes))
	}
}

func TestDaemonSession_RecordDeliveryEvent(t *testing.T) {
	// given
	ds, _ := newTestDaemonSession(t)
	result := &phonewave.DeliveryResult{
		Kind:        "specification",
		SourcePath:  "/tmp/outbox/spec.md",
		DeliveredTo: []string{"/tmp/inbox/spec.md"},
	}

	// when
	ds.RecordDeliveryEvent(result)

	// then
	events, err := ds.EventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != phonewave.EventDeliveryCompleted {
		t.Errorf("type: got %q, want %q", events[0].Type, phonewave.EventDeliveryCompleted)
	}
}

func TestDaemonSession_RecordFailureEvent(t *testing.T) {
	// given
	ds, _ := newTestDaemonSession(t)

	// when
	ds.RecordFailureEvent("/tmp/outbox/bad.md", "specification", fmt.Errorf("no route"))

	// then
	events, err := ds.EventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != phonewave.EventDeliveryFailed {
		t.Errorf("type: got %q, want %q", events[0].Type, phonewave.EventDeliveryFailed)
	}
}

func TestDaemonSession_RecordScanEvent(t *testing.T) {
	// given
	ds, _ := newTestDaemonSession(t)

	// when
	ds.RecordScanEvent("/tmp/outbox", 3, 1)

	// then
	events, err := ds.EventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != phonewave.EventScanCompleted {
		t.Errorf("type: got %q, want %q", events[0].Type, phonewave.EventScanCompleted)
	}
}

func TestDaemonSession_RecordRetryEvent(t *testing.T) {
	// given
	ds, _ := newTestDaemonSession(t)

	// when
	ds.RecordRetryEvent("retry-spec.md", "specification")

	// then
	events, err := ds.EventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != phonewave.EventErrorRetried {
		t.Errorf("type: got %q, want %q", events[0].Type, phonewave.EventErrorRetried)
	}
}

func TestDaemonSession_RecordDeliveryEvent_NilEventStore(t *testing.T) {
	// given: DaemonSession with nil EventStore
	ds, _ := newTestDaemonSession(t)
	ds.EventStore = nil

	// when: should not panic
	ds.RecordDeliveryEvent(&phonewave.DeliveryResult{
		Kind:       "specification",
		SourcePath: "/tmp/outbox/spec.md",
	})
}
