package session_test

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// testDaemonEventEmitter implements port.DaemonEventEmitter for session tests.
// It wraps the aggregate + event store without usecase import.
type testDaemonEventEmitter struct {
	agg   *domain.DeliveryAggregate
	store port.EventStore
}

func (e *testDaemonEventEmitter) EmitDelivery(sourcePath string, kind string, now time.Time) error {
	ev, err := e.agg.RecordDelivery(sourcePath, kind, now)
	if err != nil {
		return err
	}
	_, appendErr := e.store.Append(ev)
	return appendErr
}

func (e *testDaemonEventEmitter) EmitFailure(filePath string, kind string, errMsg string, now time.Time) error {
	ev, err := e.agg.RecordFailure(filePath, kind, errMsg, now)
	if err != nil {
		return err
	}
	_, appendErr := e.store.Append(ev)
	return appendErr
}

func (e *testDaemonEventEmitter) EmitScan(outboxDir string, delivered, errors int, now time.Time) error {
	ev, err := e.agg.RecordScan(outboxDir, delivered, errors, now)
	if err != nil {
		return err
	}
	_, appendErr := e.store.Append(ev)
	return appendErr
}

func (e *testDaemonEventEmitter) EmitRetry(name string, kind string, now time.Time) error {
	ev, err := e.agg.RecordRetry(name, kind, now)
	if err != nil {
		return err
	}
	_, appendErr := e.store.Append(ev)
	return appendErr
}

func newTestDaemonSession(t *testing.T) (*session.DaemonSession, port.EventStore) {
	t.Helper()
	dir := t.TempDir()
	errorQueue := testErrorQueueStore(t)
	eventStore := session.NewEventStore(dir, &domain.NopLogger{})
	dlog, err := session.NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("create delivery log: %v", err)
	}
	t.Cleanup(func() { dlog.Close() })
	logger := platform.NewLogger(io.Discard, false)
	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: "/tmp/outbox", ToInboxes: []string{"/tmp/inbox"}},
	}
	agg := domain.NewDeliveryAggregate()
	emitter := &testDaemonEventEmitter{agg: agg, store: eventStore}
	ds := session.NewDaemonSession(errorQueue, dlog, routes, dir, logger)
	ds.Emitter = emitter
	return ds, eventStore
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
	if ds.Emitter == nil {
		t.Error("Emitter should be set")
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
	ds, eventStore := newTestDaemonSession(t)
	result := &domain.DeliveryResult{
		Kind:        "specification",
		SourcePath:  "/tmp/outbox/spec.md",
		DeliveredTo: []string{"/tmp/inbox/spec.md"},
	}

	// when
	ds.RecordDeliveryEvent(result)

	// then
	events, _, err := eventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != domain.EventDeliveryCompleted {
		t.Errorf("type: got %q, want %q", events[0].Type, domain.EventDeliveryCompleted)
	}
}

func TestDaemonSession_RecordFailureEvent(t *testing.T) {
	// given
	ds, eventStore := newTestDaemonSession(t)

	// when
	ds.RecordFailureEvent("/tmp/outbox/bad.md", "specification", fmt.Errorf("no route"))

	// then
	events, _, err := eventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != domain.EventDeliveryFailed {
		t.Errorf("type: got %q, want %q", events[0].Type, domain.EventDeliveryFailed)
	}
}

func TestDaemonSession_RecordScanEvent(t *testing.T) {
	// given
	ds, eventStore := newTestDaemonSession(t)

	// when
	ds.RecordScanEvent("/tmp/outbox", 3, 1)

	// then
	events, _, err := eventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != domain.EventScanCompleted {
		t.Errorf("type: got %q, want %q", events[0].Type, domain.EventScanCompleted)
	}
}

func TestDaemonSession_RecordRetryEvent(t *testing.T) {
	// given
	ds, eventStore := newTestDaemonSession(t)

	// when
	ds.RecordRetryEvent("retry-spec.md", "specification")

	// then
	events, _, err := eventStore.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	if events[0].Type != domain.EventErrorRetried {
		t.Errorf("type: got %q, want %q", events[0].Type, domain.EventErrorRetried)
	}
}

func TestDaemonSession_RecordDeliveryEvent_NilEmitter(t *testing.T) {
	// given: DaemonSession with nil Emitter
	ds, _ := newTestDaemonSession(t)
	ds.Emitter = nil

	// when: should not panic
	ds.RecordDeliveryEvent(&domain.DeliveryResult{
		Kind:       "specification",
		SourcePath: "/tmp/outbox/spec.md",
	})
}
