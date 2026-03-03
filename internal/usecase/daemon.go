package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

// SetupAndRunDaemon validates the RunDaemonCommand, resolves configuration,
// creates a Daemon, and runs the event loop until ctx is cancelled.
func SetupAndRunDaemon(ctx context.Context, cmd domain.RunDaemonCommand, cfgPath, baseDir string, logger *domain.Logger) error {
	if errs := cmd.Validate(); len(errs) > 0 {
		return fmt.Errorf("command validation: %w", errs[0])
	}

	cfg, err := session.LoadConfig(cfgPath)
	if err != nil {
		logger.Info("Run 'phonewave init' first")
		return fmt.Errorf("load config: %w", err)
	}

	routes, err := session.ResolveRoutes(cfg)
	if err != nil {
		return fmt.Errorf("resolve routes: %w", err)
	}

	outboxDirs := session.CollectOutboxDirs(cfg)
	if len(outboxDirs) == 0 {
		logger.Warn("No outbox directories to watch")
		return nil
	}

	stateDir := filepath.Join(baseDir, domain.StateDir)
	if err := session.EnsureStateDir(baseDir); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	// Acquire daemon singleton lock before any resource allocation.
	// Prevents two daemon processes from running against the same state directory.
	// The OS releases the lock automatically if the process crashes.
	runDir := filepath.Join(stateDir, ".run")
	unlock, err := session.TryLockDaemon(runDir)
	if err != nil {
		return fmt.Errorf("daemon lock: %w", err)
	}
	defer unlock()

	// Initialize session-layer stores via factory (ADR S0008: no direct eventsource import)
	eventStore := session.NewEventStore(stateDir)

	errorQueue, err := session.NewErrorQueueStore(stateDir)
	if err != nil {
		return fmt.Errorf("create error queue store: %w", err)
	}
	defer errorQueue.Close()

	// Migrate legacy .err sidecar files to SQLite ErrorQueueStore
	migrated, migrateErr := session.MigrateFileErrorQueue(stateDir, errorQueue, logger)
	if migrateErr != nil {
		logger.Warn("Error queue migration: %v", migrateErr)
	} else if migrated > 0 {
		logger.OK("Migrated %d legacy error queue entries to SQLite", migrated)
	}

	d, err := session.NewDaemon(domain.DaemonOptions{
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

	// Inject PolicyEngine for best-effort event dispatch (ADR S0014, S0018)
	engine := NewPolicyEngine(logger)
	registerDaemonPolicies(engine, logger)
	d.Dispatcher = engine

	// Create DaemonSession for session-layer event recording
	dlog, err := session.NewDeliveryLog(stateDir)
	if err != nil {
		return fmt.Errorf("open delivery log: %w", err)
	}
	defer dlog.Close()

	ds := session.NewDaemonSession(errorQueue, eventStore, dlog, routes, stateDir, logger)
	ds.Dispatcher = engine
	_ = ds // DaemonSession available for future wiring; daemon uses it via Run hooks

	logger.OK("phonewave daemon starting (%d routes, %d outboxes)", len(routes), len(outboxDirs))

	if err := d.Run(ctx); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}

	logger.OK("Daemon stopped")
	return nil
}
