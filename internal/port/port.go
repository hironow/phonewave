package port

import (
	"context"

	"github.com/hironow/phonewave/internal/domain"
)

// EventDispatcher processes events after persistence (e.g. POLICY dispatch).
type EventDispatcher interface {
	Dispatch(ctx context.Context, event domain.Event) error
}
