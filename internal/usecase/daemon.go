package usecase

import (
	"context"
	"fmt"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// SetupAndRunDaemon validates the RunDaemonCommand, injects the PolicyEngine,
// and runs the daemon event loop until ctx is cancelled.
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
	runner.SetDispatcher(engine)

	logger.OK("phonewave daemon starting (%d routes, %d outboxes)", runner.RouteCount(), runner.OutboxCount())
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}
	logger.OK("Daemon stopped")
	return nil
}
