package usecase

// white-box-reason: policy internals: tests unexported handler registration and spy test doubles

import (
	"bytes"
	"context"
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

func TestPolicyHandler_DeliveryCompleted_InfoOutput(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, port.NopPolicyMetrics{})

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
	registerDaemonPolicies(engine, logger, spy, port.NopPolicyMetrics{})

	ev, err := domain.NewEvent(domain.EventScanCompleted, map[string]string{
		"outbox":    "/some/outbox",
		"delivered": "5",
		"errors":    "1",
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
	if !strings.Contains(call.message, "1 errors") {
		t.Errorf("expected message to contain error count, got: %s", call.message)
	}
}

func TestPolicyHandler_DeliveryCompleted_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy)

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
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy)

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
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy)

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
	registerDaemonPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventScanCompleted, map[string]string{
		"outbox":    "/some/outbox",
		"delivered": "5",
		"errors":    "1",
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
	registerDaemonPolicies(engine, logger, spy, port.NopPolicyMetrics{})

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
	registerDaemonPolicies(engine, logger, spy, port.NopPolicyMetrics{})

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
