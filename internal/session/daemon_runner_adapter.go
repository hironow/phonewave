package session

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// daemonRunnerAdapter implements port.DaemonRunner.
type daemonRunnerAdapter struct {
	daemon      *Daemon
	session     *DaemonSession
	errorQueue  port.ErrorQueueStore
	dlog        *DeliveryLog
	eventStore  port.EventStore
	unlock      func()
	notifier    port.Notifier
	insights    port.InsightAppender
	routeCount  int
	outboxCount int
}

// NewDaemonRunner creates a fully-constructed daemon runner.
// It performs all infrastructure setup: config loading, route resolution,
// state directory creation, lock acquisition, store creation, and daemon construction.
func NewDaemonRunner(cmd domain.RunDaemonCommand, cfgPath, baseDir string, logger domain.Logger) (port.DaemonRunner, error) {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		logger.Info("Run 'phonewave init' first")
		return nil, fmt.Errorf("load config: %w", err)
	}

	routes, err := ResolveRoutes(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve routes: %w", err)
	}

	outboxDirs := CollectOutboxDirs(cfg)

	stateDir := baseDir
	if err := EnsureStateDir(filepath.Dir(baseDir)); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	// Acquire daemon singleton lock before any resource allocation.
	// Prevents two daemon processes from running against the same state directory.
	// The OS releases the lock automatically if the process crashes.
	runDir := filepath.Join(stateDir, ".run")
	unlock, err := TryLockDaemon(runDir)
	if err != nil {
		return nil, fmt.Errorf("daemon lock: %w", err)
	}

	// Ensure .run/ directory exists for stores (idempotent)
	if err := EnsureRunDir(stateDir); err != nil {
		unlock()
		return nil, err
	}

	// Initialize session-layer stores via factory (ADR S0008: no direct eventsource import)
	eventStore := NewEventStore(stateDir, logger)

	errorQueue, err := NewErrorQueueStore(stateDir)
	if err != nil {
		unlock()
		return nil, fmt.Errorf("create error queue store: %w", err)
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:        routes,
		OutboxDirs:    outboxDirs,
		StateDir:      stateDir,
		Verbose:       cmd.Verbose(),
		DryRun:        cmd.DryRun(),
		RetryInterval: cmd.RetryDuration(),
		MaxRetries:    cmd.MaxRetriesInt(),
	}, logger)
	if err != nil {
		errorQueue.Close()
		unlock()
		return nil, fmt.Errorf("create daemon: %w", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		errorQueue.Close()
		unlock()
		return nil, fmt.Errorf("open delivery log: %w", err)
	}

	ds := NewDaemonSession(errorQueue, dlog, routes, stateDir, logger)
	d.session = ds

	notifier := BuildNotifier()

	insightsDir := filepath.Join(stateDir, "insights")
	insightWriter := NewInsightWriter(insightsDir, runDir)

	return &daemonRunnerAdapter{
		daemon:      d,
		session:     ds,
		errorQueue:  errorQueue,
		dlog:        dlog,
		eventStore:  eventStore,
		unlock:      unlock,
		notifier:    notifier,
		insights:    insightWriter,
		routeCount:  len(routes),
		outboxCount: len(outboxDirs),
	}, nil
}

func (a *daemonRunnerAdapter) SetEmitter(e port.DaemonEventEmitter) {
	a.session.Emitter = e
}

func (a *daemonRunnerAdapter) EventStore() port.EventStore {
	return a.eventStore
}

func (a *daemonRunnerAdapter) BuildNotifier() port.Notifier {
	return a.notifier
}

func (a *daemonRunnerAdapter) BuildInsightAppender() port.InsightAppender {
	return a.insights
}

func (a *daemonRunnerAdapter) BuildInsightReader() port.InsightReader {
	// InsightWriter already implements Read(filename) (*domain.InsightFile, error)
	if w, ok := a.insights.(*InsightWriter); ok {
		return w
	}
	return nil
}

func (a *daemonRunnerAdapter) RouteCount() int {
	return a.routeCount
}

func (a *daemonRunnerAdapter) OutboxCount() int {
	return a.outboxCount
}

func (a *daemonRunnerAdapter) Run(ctx context.Context) error {
	return a.daemon.Run(ctx)
}

func (a *daemonRunnerAdapter) Close() error {
	if a.dlog != nil {
		a.dlog.Close()
	}
	if a.errorQueue != nil {
		a.errorQueue.Close()
	}
	if a.unlock != nil {
		a.unlock()
	}
	return nil
}
