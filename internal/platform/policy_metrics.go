package platform

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelPolicyMetrics implements port.PolicyMetrics using OTel counters.
type OTelPolicyMetrics struct{}

func (*OTelPolicyMetrics) RecordPolicyEvent(ctx context.Context, eventType, status string) {
	c, err := Meter.Int64Counter("phonewave.policy.event.total",
		metric.WithDescription("Policy handler execution count"),
	)
	if err != nil {
		return
	}
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("event_type", SanitizeUTF8(eventType)),
			attribute.String("status", status), // nosemgrep: otel-attribute-string-unsanitized — caller-provided Go string constant [permanent]
		),
	)
}
