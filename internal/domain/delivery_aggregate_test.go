package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestDeliveryAggregate_RecordDelivery(t *testing.T) {
	// given
	agg := domain.NewDeliveryAggregate("")

	// when
	ev, err := agg.RecordDelivery("/outbox/test.md", "specification", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventDeliveryCompleted {
		t.Errorf("expected type %s, got %s", domain.EventDeliveryCompleted, ev.Type)
	}
}

func TestDeliveryAggregate_RecordFailure(t *testing.T) {
	// given
	agg := domain.NewDeliveryAggregate("")

	// when
	ev, err := agg.RecordFailure("/outbox/test.md", "specification", "parse error", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventDeliveryFailed {
		t.Errorf("expected type %s, got %s", domain.EventDeliveryFailed, ev.Type)
	}
}

func TestDeliveryAggregate_RecordRetry(t *testing.T) {
	// given
	agg := domain.NewDeliveryAggregate("")

	// when
	ev, err := agg.RecordRetry("retry-spec.md", "specification", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventErrorRetried {
		t.Errorf("expected type %s, got %s", domain.EventErrorRetried, ev.Type)
	}
}

func TestDeliveryAggregate_RecordScan(t *testing.T) {
	// given
	agg := domain.NewDeliveryAggregate("")

	// when
	ev, err := agg.RecordScan("/outbox", 5, 2, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventScanCompleted {
		t.Errorf("expected type %s, got %s", domain.EventScanCompleted, ev.Type)
	}
}
