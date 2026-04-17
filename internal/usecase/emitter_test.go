package usecase

// white-box-reason: emitter internals: tests unexported capturing store and event emission plumbing

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

type capturingStore struct {
	events []domain.Event
}

func (s *capturingStore) Append(_ context.Context, events ...domain.Event) (domain.AppendResult, error) {
	s.events = append(s.events, events...)
	return domain.AppendResult{}, nil
}
func (*capturingStore) LoadAll(_ context.Context) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}
func (*capturingStore) LoadSince(_ context.Context, _ time.Time) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}
func (*capturingStore) LoadAfterSeqNr(_ context.Context, _ uint64) ([]domain.Event, domain.LoadResult, error) {
	return nil, domain.LoadResult{}, nil
}
func (*capturingStore) LatestSeqNr(_ context.Context) (uint64, error) { return 0, nil }

var _ port.EventStore = (*capturingStore)(nil)

func TestEmit_EnrichesCorrelationID(t *testing.T) {
	// given: emitter with a capturing store
	store := &capturingStore{}
	agg := domain.NewDeliveryAggregate("test-delivery-id")
	emitter := &deliveryEventEmitter{
		ctx:        context.Background(),
		agg:        agg,
		store:      store,
		deliveryID: agg.ID(),
		logger:     &domain.NopLogger{},
	}

	// when
	if err := emitter.EmitDelivery("/outbox/test.md", "specification", time.Now()); err != nil {
		t.Fatalf("EmitDelivery: %v", err)
	}

	// then: CorrelationID should be set to deliveryID
	if len(store.events) == 0 {
		t.Fatal("no events captured")
	}
	ev := store.events[0]
	if ev.CorrelationID != "test-delivery-id" {
		t.Errorf("expected CorrelationID=test-delivery-id, got %q", ev.CorrelationID)
	}
}

func TestEmit_CausationChain(t *testing.T) {
	// given: emitter
	store := &capturingStore{}
	agg := domain.NewDeliveryAggregate("test-delivery-id")
	emitter := &deliveryEventEmitter{
		ctx:        context.Background(),
		agg:        agg,
		store:      store,
		deliveryID: agg.ID(),
		logger:     &domain.NopLogger{},
	}

	// when: emit two events
	if err := emitter.EmitDelivery("/outbox/a.md", "specification", time.Now()); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	if err := emitter.EmitDelivery("/outbox/b.md", "report", time.Now()); err != nil {
		t.Fatalf("second emit: %v", err)
	}

	// then: second event should have CausationID = first event's ID
	if len(store.events) < 2 {
		t.Fatalf("expected 2 events, got %d", len(store.events))
	}
	first := store.events[0]
	second := store.events[1]
	if second.CausationID != first.ID {
		t.Errorf("expected CausationID=%s, got %s", first.ID, second.CausationID)
	}
}
