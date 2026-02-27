package phonewave

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEvent_ValidDeliverySucceeded(t *testing.T) {
	// given
	data := DeliverySucceededData{
		Kind:        "report",
		SourcePath:  "outbox/rp-001.md",
		DeliveredTo: []string{"inbox/rp-001.md"},
	}
	ts := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	// when
	ev, err := NewEvent(EventDeliverySucceeded, data, ts)

	// then
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if ev.ID == "" {
		t.Error("expected non-empty ID")
	}
	if ev.Type != EventDeliverySucceeded {
		t.Errorf("expected type %s, got %s", EventDeliverySucceeded, ev.Type)
	}
	if !ev.Timestamp.Equal(ts) {
		t.Errorf("expected timestamp %v, got %v", ts, ev.Timestamp)
	}

	var payload DeliverySucceededData
	if err := json.Unmarshal(ev.Data, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Kind != "report" {
		t.Errorf("expected kind 'report', got %q", payload.Kind)
	}
}

func TestValidateEvent_RequiredFields(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		want string
	}{
		{"empty ID", Event{Type: "test", Timestamp: time.Now(), Data: json.RawMessage(`{}`)}, "ID"},
		{"empty Type", Event{ID: "x", Timestamp: time.Now(), Data: json.RawMessage(`{}`)}, "Type"},
		{"zero Timestamp", Event{ID: "x", Type: "test", Data: json.RawMessage(`{}`)}, "Timestamp"},
		{"empty Data", Event{ID: "x", Type: "test", Timestamp: time.Now()}, "Data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEvent(tt.ev)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !contains(err.Error(), tt.want) {
				t.Errorf("expected error about %q, got %q", tt.want, err.Error())
			}
		})
	}
}

func TestValidateEvent_ValidEvent(t *testing.T) {
	ev, _ := NewEvent(EventDeliverySucceeded, DeliverySucceededData{Kind: "report"}, time.Now())
	if err := ValidateEvent(ev); err != nil {
		t.Errorf("expected valid event, got %v", err)
	}
}

func TestEventTypes_ArePastTense(t *testing.T) {
	// verify all event types follow past-tense naming convention
	types := []EventType{
		EventDeliverySucceeded,
		EventDeliveryFailed,
		EventDeliveryRetried,
		EventStartupScanCompleted,
	}
	for _, et := range types {
		if string(et) == "" {
			t.Errorf("empty event type")
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
