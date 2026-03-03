package usecase

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
)

func TestPolicyHandler_DeliveryCompleted_InfoOutput(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger)

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

func TestPolicyHandler_DeliveryFailed_DebugOnly_NoInfoOutput(t *testing.T) {
	// given: delivery.failed stays Debug-only (infinite recursion prevention)
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger)

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
