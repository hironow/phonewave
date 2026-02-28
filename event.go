package phonewave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventDispatcher processes events after persistence (e.g. POLICY dispatch).
type EventDispatcher interface {
	Dispatch(ctx context.Context, event Event) error
}

// EventStore is the interface for an append-only event log.
type EventStore interface {
	Append(events ...Event) error
	LoadAll() ([]Event, error)
	LoadSince(after time.Time) ([]Event, error)
}

// EventType identifies the kind of domain event.
type EventType string

const (
	EventDeliveryCompleted EventType = "delivery.completed"
	EventDeliveryFailed    EventType = "delivery.failed"
	EventErrorRecorded     EventType = "error.recorded"
	EventErrorRetried      EventType = "error.retried"
	EventScanCompleted     EventType = "scan.completed"
)

// Event is the immutable event envelope persisted to the event store.
type Event struct {
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	Data          json.RawMessage `json:"data"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
}

// NewEvent creates an Event with a UUID, the given timestamp, and marshaled data payload.
func NewEvent(eventType EventType, data any, timestamp time.Time) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Event{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: timestamp,
		Data:      raw,
	}, nil
}

// ValidateEvent checks structural validity of an Event before persistence.
func ValidateEvent(e Event) error {
	var errs []string
	if e.ID == "" {
		errs = append(errs, "ID is required")
	}
	if e.Type == "" {
		errs = append(errs, "Type is required")
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, "Timestamp must not be zero")
	}
	if len(errs) > 0 {
		return errors.New("invalid event: " + strings.Join(errs, "; "))
	}
	return nil
}
