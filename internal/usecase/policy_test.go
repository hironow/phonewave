package usecase

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestPolicyEngine_Dispatch_NoHandlers(t *testing.T) {
	// given
	logger := domain.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	ev := domain.Event{Type: domain.EventDeliveryCompleted}

	// when
	err := engine.Dispatch(context.Background(), ev)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPolicyEngine_RegisterAndFire(t *testing.T) {
	// given
	logger := domain.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	called := false
	engine.Register(domain.EventDeliveryCompleted, func(ctx context.Context, ev domain.Event) error {
		called = true
		return nil
	})

	ev := domain.Event{Type: domain.EventDeliveryCompleted}

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
	logger := domain.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	count := 0
	handler := func(ctx context.Context, ev domain.Event) error {
		count++
		return nil
	}
	engine.Register(domain.EventDeliveryFailed, handler)
	engine.Register(domain.EventDeliveryFailed, handler)

	ev := domain.Event{Type: domain.EventDeliveryFailed}

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
	logger := domain.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	engine.Register(domain.EventErrorRetried, func(ctx context.Context, ev domain.Event) error {
		return errors.New("handler failed")
	})

	ev := domain.Event{Type: domain.EventErrorRetried}

	// when
	err := engine.Dispatch(context.Background(), ev)

	// then: best-effort — errors are logged, not propagated
	if err != nil {
		t.Fatalf("expected no error (best-effort), got %v", err)
	}
}

func TestPolicyEngine_UnmatchedEventType(t *testing.T) {
	// given
	logger := domain.NewLogger(io.Discard, false)
	engine := NewPolicyEngine(logger)
	called := false
	engine.Register(domain.EventDeliveryCompleted, func(ctx context.Context, ev domain.Event) error {
		called = true
		return nil
	})

	ev := domain.Event{Type: domain.EventScanCompleted}

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
