package domain

// white-box-reason: tests unexported seqNr field increment on DeliveryAggregate

import (
	"testing"
	"time"
)

func TestDeliveryAggregate_SeqNrIncrements(t *testing.T) {
	agg := NewDeliveryAggregate("session-1")
	now := time.Now()

	ev1, err := agg.RecordDelivery("/path/a", KindSpecification, now)
	if err != nil {
		t.Fatal(err)
	}
	ev2, err := agg.RecordFailure("/path/b", KindReport, "err", now)
	if err != nil {
		t.Fatal(err)
	}

	if ev1.SeqNr != 1 {
		t.Errorf("ev1.SeqNr = %d, want 1", ev1.SeqNr)
	}
	if ev2.SeqNr != 2 {
		t.Errorf("ev2.SeqNr = %d, want 2", ev2.SeqNr)
	}
	if ev1.AggregateID != "session-1" {
		t.Errorf("ev1.AggregateID = %q, want session-1", ev1.AggregateID)
	}
	if ev1.AggregateType != AggregateTypeDelivery {
		t.Errorf("ev1.AggregateType = %q, want %q", ev1.AggregateType, AggregateTypeDelivery)
	}
}
