package usecase

import (
	"context"
	"fmt"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// SetupAndRunDaemon validates the RunDaemonCommand, constructs the aggregate
// and EventEmitter, injects them into the runner, and runs the daemon event loop
// until ctx is cancelled.
func SetupAndRunDaemon(ctx context.Context, cmd domain.RunDaemonCommand, logger domain.Logger, metrics port.PolicyMetrics, runner port.DaemonRunner) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}
	defer runner.Close()

	if runner.OutboxCount() == 0 {
		logger.Warn("No outbox directories to watch")
		return nil
	}

	engine := NewPolicyEngine(logger)
	notifier := runner.BuildNotifier()
	if metrics == nil {
		metrics = port.NopPolicyMetrics{}
	}
	registerDaemonPolicies(engine, logger, notifier, metrics)

	// Aggregate lives in usecase — never exposed to session layer.
	agg := domain.NewDeliveryAggregate()
	emitter := NewDeliveryEventEmitter(agg, runner.EventStore(), engine, logger)
	runner.SetEmitter(emitter)

	logger.OK("phonewave daemon starting (%d routes, %d outboxes)", runner.RouteCount(), runner.OutboxCount())
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}
	logger.OK("Daemon stopped")
	return nil
}
