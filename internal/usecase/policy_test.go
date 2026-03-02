package usecase

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hironow/phonewave"
)

func TestPolicyEngine_Dispatch_NoHandlers(t *testing.T) {
	// given
	logger := phonewave.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	ev := phonewave.Event{Type: phonewave.EventDeliveryCompleted}

	// when
	err := engine.Dispatch(context.Background(), ev)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPolicyEngine_RegisterAndFire(t *testing.T) {
	// given
	logger := phonewave.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	called := false
	engine.Register(phonewave.EventDeliveryCompleted, func(ctx context.Context, ev phonewave.Event) error {
		called = true
		return nil
	})

	ev := phonewave.Event{Type: phonewave.EventDeliveryCompleted}

	// when
	err := engine.Dispatch(context.Background(), ev)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestPolicyEngine_MultipleHandlers(t *testing.T) {
	// given
	logger := phonewave.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	count := 0
	handler := func(ctx context.Context, ev phonewave.Event) error {
		count++
		return nil
	}
	engine.Register(phonewave.EventDeliveryFailed, handler)
	engine.Register(phonewave.EventDeliveryFailed, handler)

	ev := phonewave.Event{Type: phonewave.EventDeliveryFailed}

	// when
	err := engine.Dispatch(context.Background(), ev)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 calls, got %d", count)
	}
}

func TestPolicyEngine_HandlerError(t *testing.T) {
	// given
	logger := phonewave.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	engine.Register(phonewave.EventErrorRetried, func(ctx context.Context, ev phonewave.Event) error {
		return errors.New("handler failed")
	})

	ev := phonewave.Event{Type: phonewave.EventErrorRetried}

	// when
	err := engine.Dispatch(context.Background(), ev)

	// then: best-effort — errors are logged, not propagated
	if err != nil {
		t.Fatalf("expected no error (best-effort), got %v", err)
	}
}

func TestPolicyEngine_UnmatchedEventType(t *testing.T) {
	// given
	logger := phonewave.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	called := false
	engine.Register(phonewave.EventDeliveryCompleted, func(ctx context.Context, ev phonewave.Event) error {
		called = true
		return nil
	})

	ev := phonewave.Event{Type: phonewave.EventScanCompleted}

	// when
	err := engine.Dispatch(context.Background(), ev)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("handler should not have been called for unmatched event type")
	}
}
