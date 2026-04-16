package platform

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordDelivery increments the phonewave.delivery.total OTel counter.
// The counter is lazily created from the package-level Meter on each call;
// the OTel SDK deduplicates instruments internally, so this is safe and cheap.
func RecordDelivery(ctx context.Context, status, kind string) {
	c, err := Meter.Int64Counter("phonewave.delivery.total",
		metric.WithDescription("Total delivery attempts"),
	)
	if err != nil {
		return
	}
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status), // nosemgrep: otel-attribute-string-unsanitized — caller-provided Go string constant [permanent]
			attribute.String("kind", SanitizeUTF8(kind)),
		),
	)
}
