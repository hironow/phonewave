package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestNewEvent_CreatesValidEvent(t *testing.T) {
	type testPayload struct {
		Path string `json:"path"`
	}

	e, err := domain.NewEvent(domain.EventDeliveryCompleted, testPayload{Path: "/outbox/test.md"}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if e.ID == "" {
		t.Error("event ID must not be empty")
	}
	if e.Type != domain.EventDeliveryCompleted {
		t.Errorf("event type = %q, want %q", e.Type, domain.EventDeliveryCompleted)
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
		event domain.Event
	}{
		{name: "empty ID", event: domain.Event{Type: "test", Timestamp: time.Now()}},
		{name: "empty Type", event: domain.Event{ID: "abc", Timestamp: time.Now()}},
		{name: "zero Timestamp", event: domain.Event{ID: "abc", Type: "test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := domain.ValidateEvent(tt.event); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestValidateEvent_AcceptsValidEvent(t *testing.T) {
	e, _ := domain.NewEvent(domain.EventDeliveryCompleted, map[string]string{"k": "v"}, time.Now())
	if err := domain.ValidateEvent(e); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
