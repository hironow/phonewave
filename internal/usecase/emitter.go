package usecase

import (
	"context"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// deliveryEventEmitter implements port.DaemonEventEmitter.
// It wraps aggregate event production + persistence + best-effort dispatch.
type deliveryEventEmitter struct {
	agg        *domain.DeliveryAggregate
	store      port.EventStore
	seqAlloc   port.SeqAllocator // nil = no SeqNr assignment
	dispatcher port.EventDispatcher
	logger     domain.Logger
	ctx        context.Context //nolint:containedctx // stored for trace propagation into emit chain
}

// NewDeliveryEventEmitter creates a DaemonEventEmitter that wraps
// the aggregate, event store, and dispatcher into a single emit chain.
// ctx is used for trace propagation in store/dispatcher operations.
func NewDeliveryEventEmitter(
	ctx context.Context,
	agg *domain.DeliveryAggregate,
	store port.EventStore,
	dispatcher port.EventDispatcher,
	logger domain.Logger,
) port.DaemonEventEmitter {
	return &deliveryEventEmitter{ctx: ctx, agg: agg, store: store, dispatcher: dispatcher, logger: logger}
}

// SetSeqAllocator attaches a SeqAllocator for global SeqNr assignment.
func (e *deliveryEventEmitter) SetSeqAllocator(alloc port.SeqAllocator) {
	e.seqAlloc = alloc
}

// emit persists the event and dispatches it (best-effort).
func (e *deliveryEventEmitter) emit(ev domain.Event) error {
	if e.seqAlloc != nil {
		seq, err := e.seqAlloc.AllocSeqNr(e.ctx)
		if err != nil {
			return err
		}
		ev.SeqNr = seq
	}
	if e.store != nil {
		if _, err := e.store.Append(e.ctx, ev); err != nil {
			return err
		}
	}
	if e.dispatcher != nil {
		if err := e.dispatcher.Dispatch(e.ctx, ev); err != nil {
			e.logger.Warn("policy dispatch %s: %v", ev.Type, err)
		}
	}
	return nil
}

func (e *deliveryEventEmitter) EmitDelivery(sourcePath string, kind domain.DMailKind, now time.Time) error {
	ev, err := e.agg.RecordDelivery(sourcePath, kind, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *deliveryEventEmitter) EmitFailure(filePath string, kind domain.DMailKind, errMsg string, now time.Time) error {
	ev, err := e.agg.RecordFailure(filePath, kind, errMsg, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *deliveryEventEmitter) EmitScan(outboxDir string, delivered, errors int, now time.Time) error {
	ev, err := e.agg.RecordScan(outboxDir, delivered, errors, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *deliveryEventEmitter) EmitRetry(name string, kind domain.DMailKind, now time.Time) error {
	ev, err := e.agg.RecordRetry(name, kind, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}
