package phonewave

import (
	"testing"
	"time"
)

func TestDeliveryAggregate_RecordDelivery(t *testing.T) {
	// given
	agg := NewDeliveryAggregate()

	// when
	ev, err := agg.RecordDelivery("/outbox/test.md", "specification", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != EventDeliveryCompleted {
		t.Errorf("expected type %s, got %s", EventDeliveryCompleted, ev.Type)
	}
}

func TestDeliveryAggregate_RecordFailure(t *testing.T) {
	// given
	agg := NewDeliveryAggregate()

	// when
	ev, err := agg.RecordFailure("/outbox/test.md", "parse error", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != EventDeliveryFailed {
		t.Errorf("expected type %s, got %s", EventDeliveryFailed, ev.Type)
	}
}

func TestDeliveryAggregate_RecordRetry(t *testing.T) {
	// given
	agg := NewDeliveryAggregate()

	// when
	ev, err := agg.RecordRetry("/errors/test.md", 2, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != EventErrorRetried {
		t.Errorf("expected type %s, got %s", EventErrorRetried, ev.Type)
	}
}

func TestDeliveryAggregate_RecordScan(t *testing.T) {
	// given
	agg := NewDeliveryAggregate()

	// when
	ev, err := agg.RecordScan(5, 2, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != EventScanCompleted {
		t.Errorf("expected type %s, got %s", EventScanCompleted, ev.Type)
	}
}
