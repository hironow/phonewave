package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEvent_CreatesValidEvent(t *testing.T) {
	type testPayload struct {
		Path string `json:"path"`
	}

	e, err := NewEvent(EventDeliveryCompleted, testPayload{Path: "/outbox/test.md"}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.ID == "" {
		t.Error("event ID must not be empty")
	}
	if e.Type != EventDeliveryCompleted {
		t.Errorf("event type = %q, want %q", e.Type, EventDeliveryCompleted)
	}
	if e.Timestamp.IsZero() {
		t.Error("timestamp must not be zero")
	}

	var payload testPayload
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if payload.Path != "/outbox/test.md" {
		t.Errorf("payload.Path = %q, want %q", payload.Path, "/outbox/test.md")
	}
}

func TestValidateEvent_RejectsEmptyFields(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{name: "empty ID", event: Event{Type: "test", Timestamp: time.Now()}},
		{name: "empty Type", event: Event{ID: "abc", Timestamp: time.Now()}},
		{name: "zero Timestamp", event: Event{ID: "abc", Type: "test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateEvent(tt.event); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestValidateEvent_AcceptsValidEvent(t *testing.T) {
	e, _ := NewEvent(EventDeliveryCompleted, map[string]string{"k": "v"}, time.Now())
	if err := ValidateEvent(e); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
