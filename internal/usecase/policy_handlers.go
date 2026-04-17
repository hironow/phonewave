package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// registerDaemonPolicies registers POLICY handlers for daemon events.
// Handlers are best-effort: errors are logged but never stop the daemon.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerDaemonPolicies(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics, insights port.InsightAppender, reader port.InsightReader) {
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

	// POLICY: delivery.failed → notify + insight + metrics.
	// Notifier.Notify is safe: it does NOT dispatch events (no recursion).
	// InsightAppender.Append is best-effort: failures are logged, never propagated.
	engine.Register(domain.EventDeliveryFailed, func(ctx context.Context, event domain.Event) error {
		var payload domain.DeliveryFailedPayload
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			logger.Debug("policy: delivery failed parse error: %v", err)
			metrics.RecordPolicyEvent(ctx, "delivery.failed", "handled")
			return nil
		}
		logger.Info("policy: delivery failed (kind=%s, path=%s)", payload.Kind, payload.Path)

		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Phonewave", "Delivery failed"); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}

		// Write delivery failure insight (best-effort).
		writeDeliveryFailureInsight(logger, insights, reader, payload, event)

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
		var payload domain.ScanCompletedPayload
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			logger.Debug("policy: scan completed parse error: %v", err)
			return nil
		}
		logger.Info("policy: scan completed (delivered=%d, failed=%d)", payload.Delivered, payload.Failed)
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Phonewave",
			fmt.Sprintf("Scan completed: %d delivered, %d failed", payload.Delivered, payload.Failed)); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		metrics.RecordPolicyEvent(ctx, "scan.completed", "handled")
		return nil
	})
}

// categorizeDeliveryError maps error messages to human-readable categories.
func categorizeDeliveryError(errMsg string) string {
	switch {
	case containsAny(errMsg, "permission denied", "access denied"):
		return "Permission denied on target inbox directory"
	case containsAny(errMsg, "no such file", "not found", "does not exist"):
		return "Target inbox directory not found"
	case containsAny(errMsg, "no space", "disk full"):
		return "Insufficient disk space on target"
	case containsAny(errMsg, "no route", "no matching"):
		return "No matching route for D-Mail kind"
	default:
		return fmt.Sprintf("Delivery error: %s", errMsg)
	}
}

// containsAny returns true if s contains any of the substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// writeDeliveryFailureInsight creates an InsightEntry from a delivery failure
// event and appends it to the delivery.md insight file.
// Best-effort: errors are logged but never propagated.
// countRouteFailures counts prior failure entries for a given route in the insight file.
func countRouteFailures(reader port.InsightReader, route string) int {
	if reader == nil {
		return 0
	}
	file, err := reader.Read("delivery.md")
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range file.All() {
		if entry.Extra["route"] == route {
			count++
		}
	}
	return count
}

func writeDeliveryFailureInsight(logger domain.Logger, insights port.InsightAppender, reader port.InsightReader, payload domain.DeliveryFailedPayload, event domain.Event) {
	sourceOutbox := filepath.Dir(payload.Path)
	title := fmt.Sprintf("delivery-failed-%s-%s", payload.Kind, event.Timestamp.Format("20060102T150405"))
	route := fmt.Sprintf("%s -> targets", sourceOutbox)

	how := "Check target inbox directory permissions and disk space"
	if reader != nil {
		if count := countRouteFailures(reader, route); count > 0 {
			how = fmt.Sprintf("Repeated failure (%d prior) on this route — check target inbox directory permissions and disk space", count)
		}
	}

	entry := domain.InsightEntry{
		Title:       title,
		What:        fmt.Sprintf("Delivery failed for kind %s from %s: %s", payload.Kind, sourceOutbox, payload.Error),
		Why:         categorizeDeliveryError(payload.Error),
		How:         how,
		When:        "During delivery scan cycle",
		Who:         fmt.Sprintf("phonewave courier daemon (event-%s)", event.ID),
		Constraints: "Automatic retry via error queue",
		Extra: map[string]string{
			"route": route,
		},
	}

	if err := insights.Append("delivery.md", "delivery-failure", "phonewave", entry); err != nil {
		logger.Warn("policy: write delivery failure insight: %v", err)
	}
}
