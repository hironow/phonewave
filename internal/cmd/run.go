package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/service"
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
	logger := phonewave.NewLogger(cmd.ErrOrStderr(), verbose)

	cfgPath := configPath(cmd)
	cfg, err := service.LoadConfig(cfgPath)
	if err != nil {
		logger.Info("Run 'phonewave init' first")
		return fmt.Errorf("load config: %w", err)
	}

	routes, err := service.ResolveRoutes(cfg)
	if err != nil {
		return fmt.Errorf("resolve routes: %w", err)
	}

	outboxDirs := service.CollectOutboxDirs(cfg)
	if len(outboxDirs) == 0 {
		logger.Warn("No outbox directories to watch")
		return nil
	}

	base := configBase(cmd)
	stateDir := filepath.Join(base, phonewave.StateDir)
	if err := service.EnsureStateDir(base); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	d, err := service.NewDaemon(service.DaemonOptions{
		Routes:        routes,
		OutboxDirs:    outboxDirs,
		StateDir:      stateDir,
		Verbose:       verbose,
		DryRun:        dryRun,
		RetryInterval: retryInterval,
		MaxRetries:    maxRetries,
	}, logger)
	if err != nil {
		return fmt.Errorf("create daemon: %w", err)
	}

	logger.OK("phonewave daemon starting (%d routes, %d outboxes)", len(routes), len(outboxDirs))

	// Use the context from cobra's ExecuteContext — carries signal cancellation from main()
	if err := d.Run(cmd.Context()); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}

	logger.OK("Daemon stopped")
	return nil
}
