package domain

import "time"

// AggregateTypeDelivery is the aggregate type for delivery events.
const AggregateTypeDelivery = "delivery"

// DeliveryAggregate is a thin aggregate for delivery event production.
// phonewave is a reactive daemon, so the aggregate primarily provides
// a single point for event creation rather than complex domain logic.
type DeliveryAggregate struct { // nosemgrep: structure.multiple-exported-structs-go -- delivery aggregate family (DeliveryAggregate/DeliveryCompletedPayload/DeliveryFailedPayload/ErrorRetriedPayload/ScanCompletedPayload) is cohesive ES aggregate+payload set [permanent]
	id    string // daemon session identifier
	seqNr uint64
}

// NewDeliveryAggregate creates a DeliveryAggregate with the given session ID.
func NewDeliveryAggregate(sessionID string) *DeliveryAggregate {
	return &DeliveryAggregate{id: sessionID}
}

// ID returns the aggregate's delivery session identifier.
func (a *DeliveryAggregate) ID() string { return a.id }

// nextEvent creates an event tagged with this aggregate's identity and increments SeqNr.
func (a *DeliveryAggregate) nextEvent(eventType EventType, data any, now time.Time) (Event, error) {
	a.seqNr++
	ev, err := NewEvent(eventType, data, now)
	if err != nil {
		return ev, err
	}
	ev.AggregateID = a.id
	ev.AggregateType = AggregateTypeDelivery
	ev.SeqNr = a.seqNr
	return ev, nil
}

// DeliveryCompletedPayload is the payload for EventDeliveryCompleted.
type DeliveryCompletedPayload struct { // nosemgrep: structure.multiple-exported-structs-go -- delivery aggregate family; see DeliveryAggregate [permanent]
	Path string    `json:"path"`
	Kind DMailKind `json:"kind"`
}

// DeliveryFailedPayload is the payload for EventDeliveryFailed.
type DeliveryFailedPayload struct { // nosemgrep: structure.multiple-exported-structs-go -- delivery aggregate family; see DeliveryAggregate [permanent]
	Path  string    `json:"path"`
	Kind  DMailKind `json:"kind"`
	Error string    `json:"error"`
}

// ErrorRetriedPayload is the payload for EventErrorRetried.
type ErrorRetriedPayload struct { // nosemgrep: structure.multiple-exported-structs-go -- delivery aggregate family; see DeliveryAggregate [permanent]
	Name string    `json:"name"`
	Kind DMailKind `json:"kind"`
}

// ScanCompletedPayload is the payload for EventScanCompleted.
type ScanCompletedPayload struct {
	Outbox    string `json:"outbox"`
	Delivered int    `json:"delivered"`
	Failed    int    `json:"failed"`
}

// RecordDelivery produces a delivery.completed event.
func (a *DeliveryAggregate) RecordDelivery(path string, kind DMailKind, now time.Time) (Event, error) {
	return a.nextEvent(EventDeliveryCompleted, DeliveryCompletedPayload{
		Path: path,
		Kind: kind,
	}, now)
}

// RecordFailure produces a delivery.failed event.
func (a *DeliveryAggregate) RecordFailure(path string, kind DMailKind, errMsg string, now time.Time) (Event, error) {
	return a.nextEvent(EventDeliveryFailed, DeliveryFailedPayload{
		Path:  path,
		Kind:  kind,
		Error: errMsg,
	}, now)
}

// RecordRetry produces an error.retried event.
func (a *DeliveryAggregate) RecordRetry(name string, kind DMailKind, now time.Time) (Event, error) {
	return a.nextEvent(EventErrorRetried, ErrorRetriedPayload{
		Name: name,
		Kind: kind,
	}, now)
}

// RecordScan produces a scan.completed event.
func (a *DeliveryAggregate) RecordScan(outbox string, delivered, failed int, now time.Time) (Event, error) {
	return a.nextEvent(EventScanCompleted, ScanCompletedPayload{
		Outbox:    outbox,
		Delivered: delivered,
		Failed:    failed,
	}, now)
}
