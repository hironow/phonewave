package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// registerDaemonPolicies registers POLICY handlers for daemon events.
// Handlers are best-effort: errors are logged but never stop the daemon.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerDaemonPolicies(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics) {
	// POLICY CONTRACT: observation-only — log + metrics.
	// No notification needed: individual deliveries are frequent events;
	// the aggregate scan.completed handler provides user-facing notification.
	engine.Register(domain.EventDeliveryCompleted, func(ctx context.Context, event domain.Event) error {
		var data map[string]string
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: delivery completed parse error: %v", err)
			return nil
		}
		logger.Info("policy: delivery completed (kind=%s)", data["kind"])
		metrics.RecordPolicyEvent(ctx, "delivery.completed", "handled")
		return nil
	})

	// POLICY: delivery.failed → notify + metrics.
	// Notifier.Notify is safe: it does NOT dispatch events (no recursion).
	engine.Register(domain.EventDeliveryFailed, func(ctx context.Context, event domain.Event) error {
		logger.Info("policy: delivery failed (type=%s)", event.Type)
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Phonewave", "Delivery failed"); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		metrics.RecordPolicyEvent(ctx, "delivery.failed", "handled")
		return nil
	})

	// POLICY: error.retried → notify + metrics.
	engine.Register(domain.EventErrorRetried, func(ctx context.Context, event domain.Event) error {
		var data map[string]string
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: error retried parse error: %v", err)
			return nil
		}
		logger.Info("policy: error retried (name=%s, kind=%s)", data["name"], data["kind"])
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Phonewave",
			fmt.Sprintf("Error retried: %s", data["name"])); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		metrics.RecordPolicyEvent(ctx, "error.retried", "handled")
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
		metrics.RecordPolicyEvent(ctx, "scan.completed", "handled")
		return nil
	})
}
