package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/port"
)

// registerDaemonPolicies registers POLICY handlers for daemon events.
// Handlers are best-effort: errors are logged but never stop the daemon.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerDaemonPolicies(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier) {
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

	engine.Register(domain.EventScanCompleted, func(ctx context.Context, event domain.Event) error {
		var data map[string]string
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: scan completed parse error: %v", err)
			return nil
		}
		logger.Info("policy: scan completed (delivered=%s, errors=%s)", data["delivered"], data["errors"])
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Phonewave",
			fmt.Sprintf("Scan completed: %s delivered, %s errors", data["delivered"], data["errors"])); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		return nil
	})
}
