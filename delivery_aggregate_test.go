package phonewave_test

import (
	"testing"
	"time"

	"github.com/hironow/phonewave"
)

func TestDeliveryAggregate_RecordDelivery(t *testing.T) {
	// given
	agg := phonewave.NewDeliveryAggregate()

	// when
	ev, err := agg.RecordDelivery("/outbox/test.md", "specification", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != phonewave.EventDeliveryCompleted {
		t.Errorf("expected type %s, got %s", phonewave.EventDeliveryCompleted, ev.Type)
	}
}

func TestDeliveryAggregate_RecordFailure(t *testing.T) {
	// given
	agg := phonewave.NewDeliveryAggregate()

	// when
	ev, err := agg.RecordFailure("/outbox/test.md", "parse error", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != phonewave.EventDeliveryFailed {
		t.Errorf("expected type %s, got %s", phonewave.EventDeliveryFailed, ev.Type)
	}
}

func TestDeliveryAggregate_RecordRetry(t *testing.T) {
	// given
	agg := phonewave.NewDeliveryAggregate()

	// when
	ev, err := agg.RecordRetry("/errors/test.md", 2, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != phonewave.EventErrorRetried {
		t.Errorf("expected type %s, got %s", phonewave.EventErrorRetried, ev.Type)
	}
}

func TestDeliveryAggregate_RecordScan(t *testing.T) {
	// given
	agg := phonewave.NewDeliveryAggregate()

	// when
	ev, err := agg.RecordScan(5, 2, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != phonewave.EventScanCompleted {
		t.Errorf("expected type %s, got %s", phonewave.EventScanCompleted, ev.Type)
	}
}
