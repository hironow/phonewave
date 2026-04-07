package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventApplier applies domain events to update materialized projections.
type EventApplier interface {
	Apply(event Event) error
	Rebuild(events []Event) error
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// EventType identifies the kind of domain event.
type EventType string

const (
	EventDeliveryCompleted EventType = "delivery.completed"
	EventDeliveryFailed    EventType = "delivery.failed"
	EventErrorRetried      EventType = "error.retried"
	EventScanCompleted     EventType = "scan.completed"
	EventSystemCutover     EventType = "system.cutover"
)

// validEventTypes is the set of recognized EventType values.
var validEventTypes = map[EventType]bool{
	EventDeliveryCompleted: true,
	EventDeliveryFailed:    true,
	EventErrorRetried:      true,
	EventScanCompleted:     true,
	EventSystemCutover:     true,
}

// ValidEventType returns true if the given EventType is recognized.
func ValidEventType(t EventType) bool {
	return validEventTypes[t]
}

// AllValidEventTypes returns a copy of the canonical event type set (for testing).
func AllValidEventTypes() map[EventType]bool {
	cp := make(map[EventType]bool, len(validEventTypes))
	for k, v := range validEventTypes {
		cp[k] = v
	}
	return cp
}

// CurrentEventSchemaVersion is the schema version stamped on all new events.
// Legacy events (pre-Phase2) will have SchemaVersion 0 when deserialized.
const CurrentEventSchemaVersion uint8 = 1

// Event is the immutable event envelope persisted to the event store.
type Event struct {
	SchemaVersion uint8           `json:"schema_version,omitempty"`
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	Data          json.RawMessage `json:"data"`
	SessionID     string          `json:"session_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
	AggregateID   string          `json:"aggregate_id,omitempty"`
	AggregateType string          `json:"aggregate_type,omitempty"`
	SeqNr         uint64          `json:"seq_nr,omitempty"`
}

// NewEvent creates an Event with a UUID, the given timestamp, and marshaled data payload.
func NewEvent(eventType EventType, data any, timestamp time.Time) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Event{
		SchemaVersion: CurrentEventSchemaVersion,
		ID:            uuid.NewString(),
		Type:          eventType,
		Timestamp:     timestamp,
		Data:          raw,
	}, nil
}

// ErrorEntry holds a single error queue record.
type ErrorEntry struct {
	Name         string
	Data         []byte
	SourceOutbox string
	Kind         string
	OriginalName string
	ErrorMessage string
	RetryCount   int
}

// AppendResult captures metrics from an event store Append operation.
type AppendResult struct {
	BytesWritten int // total bytes written to event files
}

// LoadResult captures metrics from an event store Load operation.
type LoadResult struct {
	FileCount        int // number of .jsonl files scanned
	CorruptLineCount int // number of lines skipped due to parse errors
}

// ValidateEvent checks structural validity of an Event before persistence.
func ValidateEvent(e Event) error {
	var errs []string
	if e.SchemaVersion > CurrentEventSchemaVersion {
		errs = append(errs, fmt.Sprintf("SchemaVersion %d exceeds current %d", e.SchemaVersion, CurrentEventSchemaVersion))
	}
	if e.ID == "" {
		errs = append(errs, "ID is required")
	}
	if e.Type == "" {
		errs = append(errs, "Type is required")
	} else if !ValidEventType(e.Type) {
		errs = append(errs, fmt.Sprintf("Type %q is not a recognized event type", e.Type))
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
