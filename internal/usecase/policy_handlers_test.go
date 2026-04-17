package usecase

// white-box-reason: policy internals: tests unexported handler registration and spy test doubles

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/usecase/port"
)

type notifyCall struct {
	title   string
	message string
}

type spyNotifier struct {
	calls []notifyCall
}

type metricsCall struct {
	eventType string
	status    string
}

type spyPolicyMetrics struct {
	calls []metricsCall
}

func (s *spyPolicyMetrics) RecordPolicyEvent(_ context.Context, eventType, status string) {
	s.calls = append(s.calls, metricsCall{eventType: eventType, status: status})
}

func (s *spyNotifier) Notify(_ context.Context, title, message string) error {
	s.calls = append(s.calls, notifyCall{title: title, message: message})
	return nil
}

type insightAppendCall struct {
	filename string
	kind     string
	tool     string
	entry    domain.InsightEntry
}

type spyInsightAppender struct {
	calls []insightAppendCall
}

func (s *spyInsightAppender) Append(filename, kind, tool string, entry domain.InsightEntry) error {
	s.calls = append(s.calls, insightAppendCall{filename: filename, kind: kind, tool: tool, entry: entry})
	return nil
}

func TestPolicyHandler_DeliveryCompleted_InfoOutput(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, port.NopPolicyMetrics{}, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventDeliveryCompleted, map[string]string{
		"kind":   "specification",
		"source": "/path/to/file",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Info-level output should contain kind
	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected INFO level output, got: %s", output)
	}
	if !strings.Contains(output, "specification") {
		t.Errorf("expected kind in output, got: %s", output)
	}
}

func TestPolicyHandler_ScanCompleted_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, spy, port.NopPolicyMetrics{}, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventScanCompleted, domain.ScanCompletedPayload{
		Outbox:    "/some/outbox",
		Delivered: 5,
		Failed:    1,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Notify should have been called once
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Phonewave") {
		t.Errorf("expected title to contain 'Phonewave', got: %s", call.title)
	}
	if !strings.Contains(call.message, "5 delivered") {
		t.Errorf("expected message to contain delivered count, got: %s", call.message)
	}
	if !strings.Contains(call.message, "1 failed") {
		t.Errorf("expected message to contain failed count, got: %s", call.message)
	}
}

func TestPolicyHandler_DeliveryCompleted_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventDeliveryCompleted, map[string]string{
		"kind":   "specification",
		"source": "/path/to/file",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "delivery.completed" {
		t.Errorf("expected eventType 'delivery.completed', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_DeliveryFailed_RecordsMetrics(t *testing.T) {
	// given: RecordPolicyEvent is NOT event dispatch, so no recursion risk
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventDeliveryFailed, map[string]string{
		"kind":  "specification",
		"error": "route not found",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: exactly 1 metrics call (no recursion)
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "delivery.failed" {
		t.Errorf("expected eventType 'delivery.failed', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_ErrorRetried_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventErrorRetried, map[string]string{
		"name": "failed-001.md",
		"kind": "specification",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "error.retried" {
		t.Errorf("expected eventType 'error.retried', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_ScanCompleted_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventScanCompleted, domain.ScanCompletedPayload{
		Outbox:    "/some/outbox",
		Delivered: 5,
		Failed:    1,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "scan.completed" {
		t.Errorf("expected eventType 'scan.completed', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_DeliveryFailed_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, spy, port.NopPolicyMetrics{}, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventDeliveryFailed, map[string]string{
		"kind":  "specification",
		"error": "route not found",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Notify should have been called once
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Phonewave") {
		t.Errorf("expected title to contain 'Phonewave', got: %s", call.title)
	}
	if !strings.Contains(call.message, "failed") {
		t.Errorf("expected message to contain 'failed', got: %s", call.message)
	}
}

func TestPolicyHandler_ErrorRetried_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, spy, port.NopPolicyMetrics{}, port.NopInsightAppender{}, nil)

	ev, err := domain.NewEvent(domain.EventErrorRetried, map[string]string{
		"name": "failed-001.md",
		"kind": "specification",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Notify should have been called once
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Phonewave") {
		t.Errorf("expected title to contain 'Phonewave', got: %s", call.title)
	}
	if !strings.Contains(call.message, "failed-001.md") {
		t.Errorf("expected message to contain name, got: %s", call.message)
	}
}

func TestPolicyHandler_DeliveryFailed_WritesInsight(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyInsightAppender{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, port.NopPolicyMetrics{}, spy, nil)

	now := time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC)
	ev, err := domain.NewEvent(domain.EventDeliveryFailed, domain.DeliveryFailedPayload{
		Path:  "/repo/.siren/outbox/test.md",
		Kind:  "specification",
		Error: "permission denied",
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: InsightAppender.Append should have been called once
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Append call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if call.filename != "delivery.md" {
		t.Errorf("expected filename 'delivery.md', got: %s", call.filename)
	}
	if call.kind != "delivery-failure" {
		t.Errorf("expected kind 'delivery-failure', got: %s", call.kind)
	}
	if call.tool != "phonewave" {
		t.Errorf("expected tool 'phonewave', got: %s", call.tool)
	}
	if !strings.Contains(call.entry.Title, "delivery-failed-specification-") {
		t.Errorf("expected title to contain 'delivery-failed-specification-', got: %s", call.entry.Title)
	}
	if !strings.Contains(call.entry.What, "specification") {
		t.Errorf("expected what to contain kind, got: %s", call.entry.What)
	}
	if !strings.Contains(call.entry.What, "permission denied") {
		t.Errorf("expected what to contain error message, got: %s", call.entry.What)
	}
	if !strings.Contains(call.entry.Why, "Permission denied") {
		t.Errorf("expected why to categorize as permission denied, got: %s", call.entry.Why)
	}
	if call.entry.Who == "" {
		t.Error("expected who to be set")
	}
	if call.entry.Extra["route"] == "" {
		t.Error("expected extra 'route' to be set")
	}
}

func TestPolicyHandler_DeliveryFailed_InsightErrorCategorization(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		wantWhy string
	}{
		{
			name:    "permission denied",
			errMsg:  "open /inbox/test.md: permission denied",
			wantWhy: "Permission denied",
		},
		{
			name:    "not found",
			errMsg:  "target inbox does not exist",
			wantWhy: "Target inbox directory not found",
		},
		{
			name:    "disk full",
			errMsg:  "write /inbox/test.md: no space left on device",
			wantWhy: "Insufficient disk space",
		},
		{
			name:    "no route",
			errMsg:  "no matching route for kind feedback",
			wantWhy: "No matching route",
		},
		{
			name:    "unknown error",
			errMsg:  "something unexpected happened",
			wantWhy: "Delivery error:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			var buf bytes.Buffer
			logger := platform.NewLogger(&buf, false)
			spy := &spyInsightAppender{}
			engine := NewPolicyEngine(logger)
			registerDaemonPolicies(engine, logger, &port.NopNotifier{}, port.NopPolicyMetrics{}, spy, nil)

			ev, err := domain.NewEvent(domain.EventDeliveryFailed, domain.DeliveryFailedPayload{
				Path:  "/repo/.siren/outbox/test.md",
				Kind:  "specification",
				Error: tt.errMsg,
			}, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}

			// when
			engine.Dispatch(context.Background(), ev)

			// then
			if len(spy.calls) != 1 {
				t.Fatalf("expected 1 Append call, got %d", len(spy.calls))
			}
			if !strings.Contains(spy.calls[0].entry.Why, tt.wantWhy) {
				t.Errorf("expected why to contain %q, got: %s", tt.wantWhy, spy.calls[0].entry.Why)
			}
		})
	}
}

type stubInsightReader struct {
	entries []domain.InsightEntry
}

func (s *stubInsightReader) Read(filename string) (*domain.InsightFile, error) {
	if s == nil {
		return nil, fmt.Errorf("no reader")
	}
	f := &domain.InsightFile{}
	for _, e := range s.entries {
		f.AddEntry(e)
	}
	return f, nil
}

func TestPolicyHandler_DeliveryFailed_DetectsRepeatFailure(t *testing.T) {
	// given: 2 prior failures on the same route
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyInsightAppender{}
	route := "/repo/.siren/outbox -> targets"
	reader := &stubInsightReader{
		entries: []domain.InsightEntry{
			{Title: "prior-1", Extra: map[string]string{"route": route}},
			{Title: "prior-2", Extra: map[string]string{"route": route}},
		},
	}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, port.NopPolicyMetrics{}, spy, reader)

	ev, err := domain.NewEvent(domain.EventDeliveryFailed, domain.DeliveryFailedPayload{
		Path:  "/repo/.siren/outbox/test.md",
		Kind:  "specification",
		Error: "permission denied",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: insight How field should mention "2 prior"
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Append call, got %d", len(spy.calls))
	}
	how := spy.calls[0].entry.How
	if !strings.Contains(how, "2 prior") {
		t.Errorf("expected How to contain '2 prior', got: %s", how)
	}
	if !strings.Contains(how, "Repeated failure") {
		t.Errorf("expected How to contain 'Repeated failure', got: %s", how)
	}
}
