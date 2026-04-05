package session

import (
	"context"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"go.opentelemetry.io/otel/attribute"
)

func providerStateSpanAttrs(snapshot domain.ProviderStateSnapshot) []attribute.KeyValue {
	state := snapshot.State
	if state == "" {
		state = domain.ProviderStateActive
	}
	attrs := []attribute.KeyValue{
		attribute.String(domain.MetadataProviderState, platform.SanitizeUTF8(string(state))),
		attribute.Int(domain.MetadataProviderRetryBudget, snapshot.RetryBudget),
	}
	if snapshot.Reason != "" {
		attrs = append(attrs, attribute.String(domain.MetadataProviderReason, platform.SanitizeUTF8(snapshot.Reason)))
	}
	if !snapshot.ResumeAt.IsZero() {
		attrs = append(attrs, attribute.String(domain.MetadataProviderResumeAt, snapshot.ResumeAt.UTC().Format(time.RFC3339)))
	}
	if snapshot.ResumeCondition != "" {
		attrs = append(attrs, attribute.String(domain.MetadataProviderResumeWhen, platform.SanitizeUTF8(snapshot.ResumeCondition)))
	}
	return attrs
}

func recordRetryCycleTelemetry(ctx context.Context, successes int, snapshot domain.ProviderStateSnapshot) {
	_, span := platform.Tracer.Start(ctx, "daemon.retry_cycle")
	defer span.End()

	attrs := []attribute.KeyValue{
		attribute.Int("retry.success.count", successes),
	}
	attrs = append(attrs, providerStateSpanAttrs(snapshot)...)
	span.SetAttributes(attrs...)
}
