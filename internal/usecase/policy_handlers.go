package usecase

import (
	"context"

	"github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/domain"
)

// registerDaemonPolicies registers POLICY handlers for daemon events.
// Handlers are best-effort: errors are logged but never stop the daemon.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerDaemonPolicies(engine *PolicyEngine, logger *phonewave.Logger) {
	engine.Register(domain.EventDeliveryCompleted, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: delivery completed (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventDeliveryFailed, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: delivery failed (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventErrorRetried, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: error retried (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventScanCompleted, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: scan completed (type=%s)", event.Type)
		return nil
	})
}
