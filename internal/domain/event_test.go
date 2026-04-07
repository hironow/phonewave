package domain_test

import (
	"encoding/json"
	"regexp"
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
		{name: "empty ID", event: domain.Event{Type: domain.EventDeliveryCompleted, Timestamp: time.Now(), Data: []byte(`{}`)}},
		{name: "empty Type", event: domain.Event{ID: "abc", Timestamp: time.Now(), Data: []byte(`{}`)}},
		{name: "zero Timestamp", event: domain.Event{ID: "abc", Type: domain.EventDeliveryCompleted, Data: []byte(`{}`)}},
		{name: "empty Data", event: domain.Event{ID: "abc", Type: domain.EventDeliveryCompleted, Timestamp: time.Now()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := domain.ValidateEvent(tt.event); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestEvent_SchemaVersion_SetByNewEvent(t *testing.T) {
	ev, err := domain.NewEvent("test.event", map[string]string{"k": "v"}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if ev.SchemaVersion != domain.CurrentEventSchemaVersion {
		t.Errorf("got %d, want %d", ev.SchemaVersion, domain.CurrentEventSchemaVersion)
	}
}

func TestEvent_SchemaVersion_ZeroIsLegacy(t *testing.T) {
	raw := `{"id":"abc","type":"test","timestamp":"2026-01-01T00:00:00Z","data":{}}`
	var ev domain.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if ev.SchemaVersion != 0 {
		t.Errorf("legacy event should have SchemaVersion 0, got %d", ev.SchemaVersion)
	}
}

func TestValidateEvent_RejectsFutureSchema(t *testing.T) {
	ev, err := domain.NewEvent("test.event", map[string]string{"k": "v"}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	ev.SchemaVersion = domain.CurrentEventSchemaVersion + 1
	if err := domain.ValidateEvent(ev); err == nil {
		t.Error("expected error for future schema version")
	}
}

func TestValidateEvent_AcceptsValidEvent(t *testing.T) {
	e, _ := domain.NewEvent(domain.EventDeliveryCompleted, map[string]string{"k": "v"}, time.Now())
	if err := domain.ValidateEvent(e); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateEvent_RejectsUnknownType(t *testing.T) {
	ev, err := domain.NewEvent("totally.unknown.type", map[string]string{"k": "v"}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if err := domain.ValidateEvent(ev); err == nil {
		t.Error("expected ValidateEvent to reject unknown event type")
	}
}

func TestAllEventTypes_AreDotCase(t *testing.T) {
	dotCaseRe := regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$`)
	for et := range domain.AllValidEventTypes() {
		if !dotCaseRe.MatchString(string(et)) {
			t.Errorf("EventType %q violates dot.case naming convention", et)
		}
	}
}
