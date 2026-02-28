package phonewave

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
	Error string `json:"error"`
}

// ErrorRetriedPayload is the payload for EventErrorRetried.
type ErrorRetriedPayload struct {
	Path    string `json:"path"`
	Attempt int    `json:"attempt"`
}

// ScanCompletedPayload is the payload for EventScanCompleted.
type ScanCompletedPayload struct {
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
}

// RecordDelivery produces a delivery.completed event.
func (a *DeliveryAggregate) RecordDelivery(path, kind string, now time.Time) (Event, error) {
	return NewEvent(EventDeliveryCompleted, DeliveryCompletedPayload{
		Path: path,
		Kind: kind,
	}, now)
}

// RecordFailure produces a delivery.failed event.
func (a *DeliveryAggregate) RecordFailure(path, errMsg string, now time.Time) (Event, error) {
	return NewEvent(EventDeliveryFailed, DeliveryFailedPayload{
		Path:  path,
		Error: errMsg,
	}, now)
}

// RecordRetry produces an error.retried event.
func (a *DeliveryAggregate) RecordRetry(path string, attempt int, now time.Time) (Event, error) {
	return NewEvent(EventErrorRetried, ErrorRetriedPayload{
		Path:    path,
		Attempt: attempt,
	}, now)
}

// RecordScan produces a scan.completed event.
func (a *DeliveryAggregate) RecordScan(delivered, failed int, now time.Time) (Event, error) {
	return NewEvent(EventScanCompleted, ScanCompletedPayload{
		Delivered: delivered,
		Failed:    failed,
	}, now)
}
