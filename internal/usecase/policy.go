package usecase

import (
	"context"

	"github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/domain"
)

// PolicyHandler processes a domain event as part of a POLICY reaction.
// WHEN [EVENT] THEN [COMMAND] — handlers implement the THEN side.
type PolicyHandler func(ctx context.Context, event domain.Event) error

// PolicyEngine dispatches domain events to registered POLICY handlers.
// Dispatch is best-effort: handler errors are logged but never propagated,
// ensuring event persistence is never rolled back due to policy failures.
type PolicyEngine struct {
	handlers map[domain.EventType][]PolicyHandler
	logger   *phonewave.Logger
}

// NewPolicyEngine creates an empty PolicyEngine.
func NewPolicyEngine(logger *phonewave.Logger) *PolicyEngine {
	return &PolicyEngine{
		handlers: make(map[domain.EventType][]PolicyHandler),
		logger:   logger,
	}
}

// Register adds a handler for the given event type.
func (e *PolicyEngine) Register(trigger domain.EventType, handler PolicyHandler) {
	e.handlers[trigger] = append(e.handlers[trigger], handler)
}

// Dispatch invokes all handlers registered for the event's type.
// Errors are logged but not returned (best-effort dispatch).
func (e *PolicyEngine) Dispatch(ctx context.Context, event domain.Event) error {
	handlers, ok := e.handlers[event.Type]
	if !ok {
		return nil
	}
	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			e.logger.Debug("policy dispatch %s: %v", event.Type, err)
		}
	}
	return nil
}
