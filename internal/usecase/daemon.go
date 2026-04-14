package usecase

import (
	"context"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// PrepareDaemonRunner wires the daemon runner with EventEmitter and PolicyEngine.
// Called by cmd (composition root). After calling this, cmd invokes runner.Run(ctx).
func PrepareDaemonRunner(ctx context.Context, logger domain.Logger, metrics port.PolicyMetrics, runner port.DaemonRunner) {
	engine := NewPolicyEngine(logger)
	notifier := runner.BuildNotifier()
	if metrics == nil {
		metrics = port.NopPolicyMetrics{}
	}
	insights := runner.BuildInsightAppender()
	reader := runner.BuildInsightReader()
	registerDaemonPolicies(engine, logger, notifier, metrics, insights, reader)

	agg := domain.NewDeliveryAggregate("")
	emitter := NewDeliveryEventEmitter(ctx, agg, runner.EventStore(), engine, logger)
	runner.SetEmitter(emitter)
}
