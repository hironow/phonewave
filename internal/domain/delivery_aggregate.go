package domain

import "time"

// DeliveryAggregate is a thin aggregate for delivery event production.
// phonewave is a reactive daemon, so the aggregate primarily provides
// a single point for event creation rather than complex domain logic.
type DeliveryAggregate struct{}

// NewDeliveryAggregate creates a DeliveryAggregate.
func NewDeliveryAggregate() *DeliveryAggregate {
	return &DeliveryAggregate{}
}

// DeliveryCompletedPayload is the payload for EventDeliveryCompleted.
type DeliveryCompletedPayload struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// DeliveryFailedPayload is the payload for EventDeliveryFailed.
type DeliveryFailedPayload struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Error string `json:"error"`
}

// ErrorRetriedPayload is the payload for EventErrorRetried.
type ErrorRetriedPayload struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// ScanCompletedPayload is the payload for EventScanCompleted.
type ScanCompletedPayload struct {
	Outbox    string `json:"outbox"`
	Delivered int    `json:"delivered"`
	Failed    int    `json:"failed"`
}

// RecordDelivery produces a delivery.completed event.
func (a *DeliveryAggregate) RecordDelivery(path, kind string, now time.Time) (Event, error) {
	return NewEvent(EventDeliveryCompleted, DeliveryCompletedPayload{
		Path: path,
		Kind: kind,
	}, now)
}

// RecordFailure produces a delivery.failed event.
func (a *DeliveryAggregate) RecordFailure(path, kind, errMsg string, now time.Time) (Event, error) {
	return NewEvent(EventDeliveryFailed, DeliveryFailedPayload{
		Path:  path,
		Kind:  kind,
		Error: errMsg,
	}, now)
}

// RecordRetry produces an error.retried event.
func (a *DeliveryAggregate) RecordRetry(name, kind string, now time.Time) (Event, error) {
	return NewEvent(EventErrorRetried, ErrorRetriedPayload{
		Name: name,
		Kind: kind,
	}, now)
}

// RecordScan produces a scan.completed event.
func (a *DeliveryAggregate) RecordScan(outbox string, delivered, failed int, now time.Time) (Event, error) {
	return NewEvent(EventScanCompleted, ScanCompletedPayload{
		Outbox:    outbox,
		Delivered: delivered,
		Failed:    failed,
	}, now)
}
