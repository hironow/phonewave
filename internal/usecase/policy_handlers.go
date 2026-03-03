package usecase

import (
	"context"
	"encoding/json"

	"github.com/hironow/phonewave/internal/domain"
)

// registerDaemonPolicies registers POLICY handlers for daemon events.
// Handlers are best-effort: errors are logged but never stop the daemon.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerDaemonPolicies(engine *PolicyEngine, logger domain.Logger) {
	engine.Register(domain.EventDeliveryCompleted, func(_ context.Context, event domain.Event) error {
		var data map[string]string
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: delivery completed parse error: %v", err)
			return nil
		}
		logger.Info("policy: delivery completed (kind=%s)", data["kind"])
		return nil
	})

	// NOTE: delivery.failed handler stays Debug-only to avoid infinite recursion.
	// RecordFailureEvent dispatches delivery.failed events, so calling any
	// recording method here would cause recursive dispatch.
	engine.Register(domain.EventDeliveryFailed, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: delivery failed (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventErrorRetried, func(_ context.Context, event domain.Event) error {
		var data map[string]string
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: error retried parse error: %v", err)
			return nil
		}
		logger.Info("policy: error retried (name=%s, kind=%s)", data["name"], data["kind"])
		return nil
	})

	engine.Register(domain.EventScanCompleted, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: scan completed (type=%s)", event.Type)
		return nil
	})
}
