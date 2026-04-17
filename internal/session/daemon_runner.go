package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	pond "github.com/alitto/pond/v2"
	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/usecase/port"
)

// Daemon watches outbox directories and delivers D-Mails.
type Daemon struct {
	opts          domain.DaemonOptions
	logger        domain.Logger
	watcher       *fsnotify.Watcher
	dlog          *DeliveryLog
	deliveryStore port.DeliveryStore
	pool          pond.Pool
	eventCh       chan fsnotify.Event // buffered channel for async event processing
	session       *DaemonSession
	bloomFilter   *domain.BloomFilter    // advisory dedup filter (nil = disabled)
	cb            *platform.CircuitBreaker // delivery circuit breaker (nil = disabled)
	dedupStore    port.DeliveryDedupStore  // exact-match dedup (nil = disabled)
}

// NewDaemon creates a new Daemon with the given options and logger.
// If logger is nil, a silent logger (io.Discard) is used.
func NewDaemon(opts domain.DaemonOptions, logger domain.Logger) (*Daemon, error) {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	watcher, err := fsnotify.NewWatcher() // nosemgrep: adr0005-fsnotify-watcher-without-close — stored in Daemon struct, closed in Run() [permanent]
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &Daemon{
		opts:    opts,
		logger:  logger,
		watcher: watcher,
		pool:    pond.NewPool(runtime.NumCPU()),
		eventCh: make(chan fsnotify.Event, runtime.NumCPU()*16),
	}, nil
}

// closeDedupStore closes the dedup store if present.
func (d *Daemon) closeDedupStore() {
	if d.dedupStore != nil {
		d.dedupStore.Close()
	}
}

// Run starts the daemon event loop. It blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	defer d.pool.StopAndWait()
	defer d.watcher.Close()

	// Open delivery log
	dlog, err := NewDeliveryLog(d.opts.StateDir)
	if err != nil {
		return fmt.Errorf("open delivery log: %w", err)
	}
	d.dlog = dlog
	defer d.dlog.Close()

	// Open delivery store (Stage→Flush transactional delivery)
	ds, err := NewDeliveryStore(d.opts.StateDir)
	if err != nil {
		return fmt.Errorf("open delivery store: %w", err)
	}
	d.deliveryStore = ds
	defer d.deliveryStore.Close()

	// Load Bloom filter for delivery dedup (advisory — nil is safe)
	bf, bfErr := LoadDeliveryFilter(d.opts.StateDir)
	if bfErr != nil {
		d.logger.Warn("Load delivery filter: %v", bfErr)
	}
	if bf == nil {
		bf = domain.NewBloomFilter(10000, 0.01) // sized for ~10K deliveries at 1% FPR
	}
	d.bloomFilter = bf
	defer func() {
		if saveErr := SaveDeliveryFilter(d.opts.StateDir, d.bloomFilter); saveErr != nil {
			d.logger.Warn("Save delivery filter: %v", saveErr)
		}
	}()

	// Recover unflushed deliveries from crash-interrupted sessions
	unflushed, recoverErr := ds.RecoverUnflushed()
	if recoverErr != nil {
		d.logger.Warn("Recover unflushed: %v", recoverErr)
	} else if len(unflushed) > 0 {
		d.logger.Info("Recovering %d unflushed deliveries from previous session", len(unflushed))
		flushed, flushErr := ds.FlushDeliveries(ctx)
		if flushErr != nil {
			d.logger.Warn("Flush recovered deliveries: %v", flushErr)
		}
		if d.opts.Verbose && len(flushed) > 0 {
			d.logger.OK("Recovered %d deliveries", len(flushed))
		}
	}

	// Register watchers on all outbox directories
	for _, dir := range d.opts.OutboxDirs {
		if err := d.watcher.Add(dir); err != nil {
			return fmt.Errorf("watch %s: %w", dir, err)
		}
		if d.opts.Verbose {
			d.logger.Info("Watching %s", dir)
		}
	}

	// Write PID file
	pidPath := filepath.Join(d.opts.StateDir, "watch.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}
	defer os.Remove(pidPath)

	// Write start timestamp for uptime tracking
	startedPath := filepath.Join(d.opts.StateDir, "watch.started")
	if err := os.WriteFile(startedPath, []byte(time.Now().UTC().Format(time.RFC3339)), 0644); err != nil {
		return fmt.Errorf("write started file: %w", err)
	}
	defer os.Remove(startedPath)

	d.runStartupScan(ctx)

	if d.opts.Verbose {
		d.logger.OK("Daemon started (PID %d), watching %d outbox directories", os.Getpid(), len(d.opts.OutboxDirs))
	}

	// Start worker goroutine that drains eventCh and processes events
	// asynchronously via the pool. This decouples fsnotify event reception
	// from the (potentially slow) handleEvent I/O, preventing event loop
	// stalls during delivery bursts.
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for event := range d.eventCh {
			d.handleEvent(ctx, event)
		}
	}()

	// Optional retry timer with exponential backoff (nil channel disables the case)
	var retryCh <-chan time.Time
	var retryTimer *time.Timer
	var backoff *domain.RetryBackoff
	if d.opts.RetryInterval > 0 {
		backoff = domain.NewRetryBackoff(d.opts.RetryInterval, d.opts.RetryInterval*32)
		retryTimer = time.NewTimer(backoff.Next())
		retryCh = retryTimer.C
		defer retryTimer.Stop()

		// Initialize delivery circuit breaker alongside retry
		d.cb = platform.NewCircuitBreaker(d.logger)
	}
	if writeErr := writeProviderStateSnapshot(d.opts.StateDir, domain.ActiveProviderState()); writeErr != nil {
		d.logger.Warn("Write provider state: %v", writeErr)
	}

	// Optional idle timer (nil channel disables the case).
	// Exits the daemon cleanly when no activity occurs for the configured duration.
	var idleCh <-chan time.Time
	var idleTimer *time.Timer
	effectiveIdle := domain.EffectiveIdleTimeout(d.opts.IdleTimeout)
	if effectiveIdle > 0 {
		idleTimer = time.NewTimer(effectiveIdle)
		idleCh = idleTimer.C
		defer idleTimer.Stop()
		if d.opts.Verbose {
			d.logger.Info("Idle timeout: %s", effectiveIdle)
		}
	}

	// resetIdle resets the idle timer on any activity.
	resetIdle := func() {
		if idleTimer != nil {
			idleTimer.Reset(effectiveIdle)
		}
	}

	// Event loop: receives fsnotify events and enqueues them for the worker.
	for {
		select {
		case <-ctx.Done():
			if d.opts.Verbose {
				d.logger.Info("Shutting down daemon")
			}
			close(d.eventCh)
			<-workerDone // wait for in-flight event processing to finish
			return nil

		case event, ok := <-d.watcher.Events:
			if !ok {
				close(d.eventCh)
				<-workerDone
				return nil
			}
			resetIdle()
			d.eventCh <- event

		case err, ok := <-d.watcher.Errors:
			if !ok {
				close(d.eventCh)
				<-workerDone
				return nil
			}
			d.logger.Warn("Watcher error: %v", err)

		case <-retryCh:
			resetIdle()
			// Gate retry cycle on circuit breaker: skip if open
			if d.cb != nil {
				if cbErr := d.cb.Allow(ctx); cbErr != nil {
					retryTimer.Reset(backoff.Next())
					continue
				}
			}
			successes := d.retryPending(ctx)
			if successes > 0 {
				backoff.RecordSuccess()
				if d.cb != nil {
					d.cb.RecordSuccess()
				}
			} else {
				backoff.RecordFailure()
				if d.cb != nil && d.hasErrorQueue() {
					// Zero successes with pending entries → target degradation
					pending, pendingErr := d.errorQueueStore().PendingCount(d.opts.MaxRetries)
				if pendingErr != nil {
					pending = 0
				}
					if pending > 0 {
						d.cb.RecordDeliveryError(domain.DeliveryErrorInfo{Kind: domain.DeliveryErrorTransient})
					}
				}
			}
			// Prefer CB snapshot when available, fall back to backoff snapshot
			var snapshot domain.ProviderStateSnapshot
			if d.cb != nil {
				snapshot = d.cb.Snapshot()
			} else {
				snapshot = backoff.Snapshot()
			}
			recordRetryCycleTelemetry(ctx, successes, snapshot)
			if writeErr := writeProviderStateSnapshot(d.opts.StateDir, snapshot); writeErr != nil {
				d.logger.Warn("Write provider state: %v", writeErr)
			}
			retryTimer.Reset(backoff.Next())

		case <-idleCh:
			d.logger.Info("No activity for %s. Exiting.", effectiveIdle)
			close(d.eventCh)
			<-workerDone
			return nil
		}
	}
}

// --- Forwarding methods: eliminate 2-level d.session.Method() access ---

func (d *Daemon) hasErrorQueue() bool {
	return d.session.HasErrorQueue()
}

func (d *Daemon) errorQueueStore() port.ErrorQueueStore {
	if d.session == nil {
		return nil
	}
	return d.session.ErrorQueueStore()
}

func (d *Daemon) enqueueError(name string, data []byte, meta domain.ErrorMetadata) error {
	return d.session.EnqueueError(name, data, meta)
}

func (d *Daemon) claimPendingRetries(claimerID string, maxRetries int) ([]domain.ErrorEntry, error) {
	return d.session.ClaimPendingRetries(claimerID, maxRetries)
}

func (d *Daemon) incrementRetry(name string, newError string) error {
	return d.session.IncrementRetry(name, newError)
}

func (d *Daemon) markResolved(name string) error {
	return d.session.MarkResolved(name)
}

func (d *Daemon) recordDeliveryEvent(ctx context.Context, result *domain.DeliveryResult) {
	if d.session == nil {
		return
	}
	d.session.RecordDeliveryEvent(ctx, result)
}

func (d *Daemon) recordFailureEvent(ctx context.Context, filePath string, kind domain.DMailKind, deliverErr error) {
	if d.session == nil {
		return
	}
	d.session.RecordFailureEvent(ctx, filePath, kind, deliverErr)
}

func (d *Daemon) recordRetryEvent(name string, kind domain.DMailKind) {
	if d.session == nil {
		return
	}
	d.session.RecordRetryEvent(name, kind)
}

// shouldProcessEvent returns true if the fsnotify event should be processed.
// It filters for Create/Rename events of deliverable D-Mail files.
func shouldProcessEvent(event fsnotify.Event) bool {
	if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
		return false
	}
	return domain.IsDMailFile(event.Name)
}

// runStartupScan delivers files that accumulated while the daemon was down.
// Each outbox directory is scanned concurrently via the daemon's worker pool.
func (d *Daemon) runStartupScan(ctx context.Context) {
	scanGroup := d.pool.NewGroup()
	for _, dir := range d.opts.OutboxDirs {
		scanGroup.Submit(func() {
			scanCtx, scanSpan := platform.Tracer.Start(ctx, "daemon.startup_scan", // nosemgrep: adr0003-otel-span-without-defer-end — span.End() called explicitly after SetAttributes [permanent]
				trace.WithNewRoot(),
				trace.WithAttributes(attribute.String("outbox.dir", platform.SanitizeUTF8(dir))),
			)
			var eq port.ErrorQueueStore
			if d.session != nil {
				eq = d.errorQueueStore()
			}
			results, errs := ScanAndDeliver(scanCtx, dir, d.opts.Routes, d.opts.StateDir, d.logger, d.deliveryStore, eq, d.bloomFilter, d.dedupStore)
			scanSpan.SetAttributes(attribute.Int("delivered.count", len(results)))
			scanSpan.End()
			for _, r := range results {
				if d.dlog != nil {
					for _, target := range r.DeliveredTo {
						d.dlog.Delivered(string(r.Kind), r.SourcePath, target)
					}
					// Only log REMOVED if source was actually removed (all targets flushed)
					if _, statErr := os.Stat(r.SourcePath); errors.Is(statErr, os.ErrNotExist) {
						d.dlog.Removed(r.SourcePath)
					}
				}
				// Mark as delivered in Bloom filter for future dedup
				if d.bloomFilter != nil && len(r.DeliveredTo) > 0 {
					d.bloomFilter.Add(r.SourcePath)
				}
				if d.opts.Verbose {
					d.logger.OK("Startup: delivered %s (kind=%s) to %v", r.SourcePath, r.Kind, r.DeliveredTo)
				}
			}
			for _, err := range errs {
				d.logger.Warn("Startup scan: %v", err)
			}
		})
	}
	scanGroup.Wait()
}
