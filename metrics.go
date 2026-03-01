package phonewave

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// DeliveryMetrics holds delivery counts for success rate calculation.
type DeliveryMetrics struct {
	Delivered int
	Failed    int
	Retried   int
}

// SuccessRate calculates the delivery success rate.
// Retried deliveries count as successes (they eventually delivered).
// Returns 0.0 if there are no deliveries.
func (m DeliveryMetrics) SuccessRate() float64 {
	total := m.Delivered + m.Failed
	if total == 0 {
		return 0.0
	}
	return float64(m.Delivered) / float64(total)
}

// RecordDelivery increments the phonewave.delivery.total OTel counter.
// The counter is lazily created from the package-level Meter on each call;
// the OTel SDK deduplicates instruments internally, so this is safe and cheap.
func RecordDelivery(ctx context.Context, status, kind string) {
	c, _ := Meter.Int64Counter("phonewave.delivery.total",
		metric.WithDescription("Total delivery attempts"),
	)
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("kind", kind),
		),
	)
}

// FormatSuccessRate formats a delivery success rate as a human-readable string.
// Returns "no deliveries" when total is 0.
func FormatSuccessRate(rate float64, success, total int) string {
	if total == 0 {
		return "no deliveries"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", rate*100, success, total)
}
