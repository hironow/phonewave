package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave"
)

// SetupAndRunDaemon validates the RunDaemonCommand, resolves configuration,
// creates a Daemon, and runs the event loop until ctx is cancelled.
func SetupAndRunDaemon(ctx context.Context, cmd phonewave.RunDaemonCommand, cfgPath, baseDir string, logger *phonewave.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}

	cfg, err := phonewave.LoadConfig(cfgPath)
	if err != nil {
		logger.Info("Run 'phonewave init' first")
		return fmt.Errorf("load config: %w", err)
	}

	routes, err := phonewave.ResolveRoutes(cfg)
	if err != nil {
		return fmt.Errorf("resolve routes: %w", err)
	}

	outboxDirs := phonewave.CollectOutboxDirs(cfg)
	if len(outboxDirs) == 0 {
		logger.Warn("No outbox directories to watch")
		return nil
	}

	stateDir := filepath.Join(baseDir, phonewave.StateDir)
	if err := phonewave.EnsureStateDir(baseDir); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	d, err := phonewave.NewDaemon(phonewave.DaemonOptions{
		Routes:        routes,
		OutboxDirs:    outboxDirs,
		StateDir:      stateDir,
		Verbose:       cmd.Verbose,
		DryRun:        cmd.DryRun,
		RetryInterval: cmd.RetryInterval,
		MaxRetries:    cmd.MaxRetries,
	}, logger)
	if err != nil {
		return fmt.Errorf("create daemon: %w", err)
	}

	// Inject PolicyEngine for best-effort event dispatch
	engine := NewPolicyEngine(logger)
	d.Dispatcher = engine

	logger.OK("phonewave daemon starting (%d routes, %d outboxes)", len(routes), len(outboxDirs))

	if err := d.Run(ctx); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}

	logger.OK("Daemon stopped")
	return nil
}
