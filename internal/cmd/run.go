package cmd

import (
	"fmt"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
	"github.com/hironow/phonewave/internal/usecase"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the courier daemon",
		Long:  "Start the phonewave courier daemon. Watches outbox directories for new D-Mails and delivers them to the correct inbox(es) based on the routing table.",
		Args:  cobra.NoArgs,
		Example: `  # Start daemon (foreground, verbose)
  phonewave run -v

  # Dry run (detect events, don't deliver)
  phonewave run -n

  # With retry interval
  phonewave run -r 120s

  # With tracing enabled
  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 phonewave run -v`,
		RunE: runDaemon,
	}

	cmd.Flags().BoolP("dry-run", "n", false, "Detect events without delivering")
	cmd.Flags().DurationP("retry-interval", "r", 60*time.Second, "Error queue retry interval (0 to disable)")
	cmd.Flags().IntP("max-retries", "m", 10, "Maximum retry attempts per failed D-Mail")
	cmd.Flags().Duration("idle-timeout", domain.DefaultIdleTimeout, "idle timeout — exit after no activity (0 = 24h safety cap, negative = disable)")

	return cmd
}

func runDaemon(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	retryInterval, _ := cmd.Flags().GetDuration("retry-interval")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	idleTimeout, _ := cmd.Flags().GetDuration("idle-timeout")
	logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)

	// Parse raw inputs into domain primitives
	ri, err := domain.NewRetryInterval(retryInterval)
	if err != nil {
		return err
	}
	mr, err := domain.NewMaxRetries(maxRetries)
	if err != nil {
		return err
	}

	daemonCmd := domain.NewRunDaemonCommand(verbose, dryRun, ri, mr, idleTimeout)

	runner, err := session.NewDaemonRunner(daemonCmd, configPath(cmd), configBase(cmd), logger)
	if err != nil {
		return err
	}

	defer runner.Close()

	if runner.OutboxCount() == 0 {
		logger.Warn("No outbox directories to watch")
		return nil
	}

	usecase.PrepareDaemonRunner(cmd.Context(), logger, &platform.OTelPolicyMetrics{}, runner)

	logger.OK("phonewave daemon starting (%d routes, %d outboxes)", runner.RouteCount(), runner.OutboxCount())
	if err := runner.Run(cmd.Context()); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}
	logger.OK("Daemon stopped")
	return nil
}
