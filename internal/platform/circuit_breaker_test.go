package platform_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
)

func TestCircuitBreaker_AllowWhenClosed(t *testing.T) {
	// given
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})

	// when
	err := cb.Allow(context.Background())

	// then
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCircuitBreaker_TransientErrorTrips(t *testing.T) {
	// given
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := domain.DeliveryErrorInfo{Kind: domain.DeliveryErrorTransient}

	// when
	cb.RecordDeliveryError(info)

	// then
	if !cb.IsOpen() {
		t.Error("expected circuit breaker to be open after transient error")
	}
	if cb.String() != "OPEN (backoff)" {
		t.Errorf("unexpected state string: %s", cb.String())
	}
}

func TestCircuitBreaker_PersistentErrorDoesNotTrip(t *testing.T) {
	// given
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	info := domain.DeliveryErrorInfo{Kind: domain.DeliveryErrorPersistent}

	// when
	cb.RecordDeliveryError(info)

	// then
	if cb.IsOpen() {
		t.Error("expected circuit breaker to stay closed for persistent error")
	}
}

func TestCircuitBreaker_RecordSuccessResets(t *testing.T) {
	// given
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	cb.RecordDeliveryError(domain.DeliveryErrorInfo{Kind: domain.DeliveryErrorTransient})

	// when
	cb.RecordSuccess()

	// then
	if cb.IsOpen() {
		t.Error("expected circuit breaker to be closed after success")
	}
	if cb.String() != "CLOSED" {
		t.Errorf("unexpected state: %s", cb.String())
	}
}

func TestCircuitBreaker_SnapshotMapsToProviderState(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(cb *platform.CircuitBreaker)
		wantState domain.ProviderState
	}{
		{
			name:      "closed maps to active",
			setup:     func(_ *platform.CircuitBreaker) {},
			wantState: domain.ProviderStateActive,
		},
		{
			name: "open maps to waiting",
			setup: func(cb *platform.CircuitBreaker) {
				cb.RecordDeliveryError(domain.DeliveryErrorInfo{Kind: domain.DeliveryErrorTransient})
			},
			wantState: domain.ProviderStateWaiting,
		},
		{
			name: "open with reset time maps to paused",
			setup: func(cb *platform.CircuitBreaker) {
				cb.RecordDeliveryError(domain.DeliveryErrorInfo{
					Kind:    domain.DeliveryErrorTransient,
					ResetAt: time.Now().Add(5 * time.Minute),
				})
			},
			wantState: domain.ProviderStatePaused,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			cb := platform.NewCircuitBreaker(&domain.NopLogger{})
			tt.setup(cb)

			// when
			snapshot := cb.Snapshot()

			// then
			if snapshot.State != tt.wantState {
				t.Errorf("snapshot state = %s, want %s", snapshot.State, tt.wantState)
			}
		})
	}
}

func TestCircuitBreaker_AllowBlocksAndRecovers(t *testing.T) {
	// given — CB is open, but backoff will elapse quickly (we can't control backoff base,
	// so we record success from another goroutine to unblock)
	cb := platform.NewCircuitBreaker(&domain.NopLogger{})
	cb.RecordDeliveryError(domain.DeliveryErrorInfo{Kind: domain.DeliveryErrorTransient})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// when — record success after a short delay to trigger state change
	go func() {
		time.Sleep(100 * time.Millisecond)
		cb.RecordSuccess()
	}()

	err := cb.Allow(ctx)

	// then
	if err != nil {
		t.Errorf("expected Allow to succeed after recovery, got %v", err)
	}
}
