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
	dispatcher port.EventDispatcher
	logger     domain.Logger
}

// NewDeliveryEventEmitter creates a DaemonEventEmitter that wraps
// the aggregate, event store, and dispatcher into a single emit chain.
// Dispatch is best-effort: errors are logged but not returned.
func NewDeliveryEventEmitter(
	agg *domain.DeliveryAggregate,
	store port.EventStore,
	dispatcher port.EventDispatcher,
	logger domain.Logger,
) port.DaemonEventEmitter {
	return &deliveryEventEmitter{agg: agg, store: store, dispatcher: dispatcher, logger: logger}
}

// emit persists the event and dispatches it (best-effort).
func (e *deliveryEventEmitter) emit(ev domain.Event) error {
	if e.store != nil {
		if err := e.store.Append(ev); err != nil {
			return err
		}
	}
	if e.dispatcher != nil {
		if err := e.dispatcher.Dispatch(context.Background(), ev); err != nil {
			e.logger.Warn("policy dispatch %s: %v", ev.Type, err)
		}
	}
	return nil
}

func (e *deliveryEventEmitter) EmitDelivery(sourcePath string, kind string, now time.Time) error {
	ev, err := e.agg.RecordDelivery(sourcePath, kind, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}

func (e *deliveryEventEmitter) EmitFailure(filePath string, kind string, errMsg string, now time.Time) error {
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

func (e *deliveryEventEmitter) EmitRetry(name string, kind string, now time.Time) error {
	ev, err := e.agg.RecordRetry(name, kind, now)
	if err != nil {
		return err
	}
	return e.emit(ev)
}
