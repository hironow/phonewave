package usecase

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/port"
)

type notifyCall struct {
	title   string
	message string
}

type spyNotifier struct {
	calls []notifyCall
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
	registerDaemonPolicies(engine, logger, &port.NopNotifier{})

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
	registerDaemonPolicies(engine, logger, spy)

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

func TestPolicyHandler_DeliveryFailed_DebugOnly_NoInfoOutput(t *testing.T) {
	// given: delivery.failed stays Debug-only (infinite recursion prevention)
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger, &port.NopNotifier{})

	ev, err := domain.NewEvent(domain.EventDeliveryFailed, map[string]string{
		"kind":  "specification",
		"error": "route not found",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: no output (Debug suppressed when verbose=false)
	output := buf.String()
	if output != "" {
		t.Errorf("expected no output for Debug-only handler with verbose=false, got: %s", output)
	}
}
