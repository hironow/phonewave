package cmd

import (
	"time"

	"github.com/hironow/phonewave/internal/domain"
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

	return cmd
}

func runDaemon(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	retryInterval, _ := cmd.Flags().GetDuration("retry-interval")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	logger := domain.NewLogger(cmd.ErrOrStderr(), verbose)

	return usecase.SetupAndRunDaemon(cmd.Context(), domain.RunDaemonCommand{
		Verbose:       verbose,
		DryRun:        dryRun,
		RetryInterval: retryInterval,
		MaxRetries:    maxRetries,
	}, configPath(cmd), configBase(cmd), logger)
}
