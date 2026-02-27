package phonewave

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventStore is the append-only event persistence interface.
type EventStore interface {
	// Append persists one or more events. Validation is performed before any writes.
	Append(events ...Event) error

	// LoadAll returns all events in chronological order.
	LoadAll() ([]Event, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]Event, error)
}

// EventType identifies the kind of domain event.
type EventType string

const (
	EventDeliverySucceeded    EventType = "delivery.succeeded"
	EventDeliveryFailed       EventType = "delivery.failed"
	EventDeliveryRetried      EventType = "delivery.retried"
	EventStartupScanCompleted EventType = "startup_scan.completed"
)

// Event is the envelope for all domain events in the event store.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// ValidateEvent checks that an Event has all required fields populated.
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
	if len(e.Data) == 0 {
		errs = append(errs, "Data must not be empty")
	}
	if len(errs) > 0 {
		return errors.New("invalid event: " + strings.Join(errs, "; "))
	}
	return nil
}

// NewEvent creates a new Event with a UUID, the given timestamp, and marshaled data payload.
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

// DeliverySucceededData is the payload for EventDeliverySucceeded.
type DeliverySucceededData struct {
	Kind        string   `json:"kind"`
	SourcePath  string   `json:"source_path"`
	DeliveredTo []string `json:"delivered_to"`
}

// DeliveryFailedData is the payload for EventDeliveryFailed.
type DeliveryFailedData struct {
	Kind       string `json:"kind"`
	SourcePath string `json:"source_path"`
	Reason     string `json:"reason"`
	Attempt    int    `json:"attempt"`
}

// DeliveryRetriedData is the payload for EventDeliveryRetried.
type DeliveryRetriedData struct {
	Kind        string   `json:"kind"`
	SourcePath  string   `json:"source_path"`
	DeliveredTo []string `json:"delivered_to"`
	Attempt     int      `json:"attempt"`
}

// StartupScanCompletedData is the payload for EventStartupScanCompleted.
type StartupScanCompletedData struct {
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
}
