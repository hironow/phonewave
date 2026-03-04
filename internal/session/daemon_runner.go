package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	pond "github.com/alitto/pond/v2"
	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/port"
)

// Daemon watches outbox directories and delivers D-Mails.
type Daemon struct {
	opts          domain.DaemonOptions
	logger        domain.Logger
	watcher       *fsnotify.Watcher
	dlog          *DeliveryLog
	deliveryStore domain.DeliveryStore
	pool          pond.Pool
	eventCh       chan fsnotify.Event // buffered channel for async event processing
	Dispatcher    port.EventDispatcher
	Session       *DaemonSession
}

// NewDaemon creates a new Daemon with the given options and logger.
// If logger is nil, a silent logger (io.Discard) is used.
func NewDaemon(opts domain.DaemonOptions, logger domain.Logger) (*Daemon, error) {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	watcher, err := fsnotify.NewWatcher() // nosemgrep: adr0005-fsnotify-watcher-without-close — stored in Daemon struct, closed in Run()
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

	// Recover unflushed deliveries from crash-interrupted sessions
	unflushed, recoverErr := ds.RecoverUnflushed()
	if recoverErr != nil {
		d.logger.Warn("Recover unflushed: %v", recoverErr)
	} else if len(unflushed) > 0 {
		d.logger.Info("Recovering %d unflushed deliveries from previous session", len(unflushed))
		flushed, flushErr := ds.FlushDeliveries()
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

	// Startup scan: deliver any files that accumulated while daemon was down.
	// Each outbox directory is scanned concurrently via the daemon's worker pool.
	scanGroup := d.pool.NewGroup()
	for _, dir := range d.opts.OutboxDirs {
		scanGroup.Submit(func() {
			scanCtx, scanSpan := platform.Tracer.Start(ctx, "daemon.startup_scan", // nosemgrep: adr0003-otel-span-without-defer-end — span.End() called explicitly after SetAttributes
				trace.WithNewRoot(),
				trace.WithAttributes(attribute.String("outbox.dir", dir)),
			)
			var eq domain.ErrorQueueStore
			if d.Session != nil {
				eq = d.Session.ErrorQueue
			}
			results, errs := ScanAndDeliver(scanCtx, dir, d.opts.Routes, d.opts.StateDir, d.logger, d.deliveryStore, eq)
			scanSpan.SetAttributes(attribute.Int("delivered.count", len(results)))
			scanSpan.End()
			for _, r := range results {
				if d.dlog != nil {
					for _, target := range r.DeliveredTo {
						d.dlog.Delivered(r.Kind, r.SourcePath, target)
					}
					// Only log REMOVED if source was actually removed (all targets flushed)
					if _, statErr := os.Stat(r.SourcePath); errors.Is(statErr, os.ErrNotExist) {
						d.dlog.Removed(r.SourcePath)
					}
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
			d.handleEvent(event)
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
			d.eventCh <- event

		case err, ok := <-d.watcher.Errors:
			if !ok {
				close(d.eventCh)
				<-workerDone
				return nil
			}
			d.logger.Warn("Watcher error: %v", err)

		case <-retryCh:
			successes := d.retryPending()
			if successes > 0 {
				backoff.RecordSuccess()
			} else {
				backoff.RecordFailure()
			}
			retryTimer.Reset(backoff.Next())
		}
	}
}

// handleEvent processes a single fsnotify event.
func (d *Daemon) handleEvent(event fsnotify.Event) {
	// React to Create and Rename events for .md files.
	// Rename is needed because producers using atomic temp+rename
	// may only emit Rename (not Create) on some platforms (e.g. Linux inotify).
	if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
		return
	}

	name := filepath.Base(event.Name)
	if filepath.Ext(name) != ".md" {
		return
	}
	// Skip temp files
	if strings.HasPrefix(name, ".phonewave-tmp-") {
		return
	}

	ctx, span := platform.Tracer.Start(context.Background(), "daemon.handle_event",
		trace.WithAttributes(
			attribute.String("event.name", event.Name),
			attribute.String("event.op", event.Op.String()),
		),
	)
	defer span.End()

	// Small delay to let the file be fully written
	time.Sleep(50 * time.Millisecond)

	if d.opts.DryRun {
		d.logger.Info("[dry-run] Detected %s", event.Name)
		return
	}

	// Read file content upfront (needed for error queue on failure).
	// Silently ignore ErrNotExist: Rename events fire for both the
	// source (gone) and target (arrived) paths.
	data, readErr := os.ReadFile(event.Name)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return
		}
		d.logger.Error("Read %s: %v", event.Name, readErr)
		span.RecordError(readErr)
		span.SetStatus(codes.Error, readErr.Error())
		return
	}

	result, err := DeliverData(ctx, event.Name, data, d.opts.Routes, d.deliveryStore)
	if err != nil {
		kind, _ := domain.ExtractDMailKind(data)
		if kind == "" { kind = "unknown" }
		d.logger.Error("Deliver %s: %v", event.Name, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if d.dlog != nil {
			d.dlog.Failed(kind, event.Name, err.Error())
		}
		if d.Session != nil {
			d.Session.RecordFailureEvent(event.Name, kind, err)
		}

		meta := domain.ErrorMetadata{
			SourceOutbox: filepath.Dir(event.Name),
			Kind:         kind,
			OriginalName: filepath.Base(event.Name),
			Attempts:     1,
			Error:        err.Error(),
			Timestamp:    time.Now().UTC(),
		}
		if d.Session == nil || d.Session.ErrorQueue == nil {
			d.logger.Error("Error queue unavailable, leaving in outbox for next startup")
			return
		}
		name := fmt.Sprintf("%s-%s-%s", meta.Timestamp.Format("2006-01-02T150405.000000000"), meta.Kind, meta.OriginalName)
		if saveErr := d.Session.ErrorQueue.Enqueue(name, data, meta); saveErr != nil {
			d.logger.Error("Error queue enqueue: %v", saveErr)
			return
		}

		// Error queue write succeeded — safe to remove from outbox
		os.Remove(event.Name)
		return
	}

	if d.dlog != nil {
		for _, target := range result.DeliveredTo {
			d.dlog.Delivered(result.Kind, result.SourcePath, target)
		}
		// Only log REMOVED if source was actually removed (all targets flushed)
		if _, statErr := os.Stat(result.SourcePath); errors.Is(statErr, os.ErrNotExist) {
			d.dlog.Removed(result.SourcePath)
		}
	}
	if d.Session != nil {
		d.Session.RecordDeliveryEvent(result)
	}

	if d.opts.Verbose {
		d.logger.OK("Delivered %s (kind=%s) to %v", result.SourcePath, result.Kind, result.DeliveredTo)
	}
}

// retryPending claims pending error queue entries via SQLite and attempts
// to re-deliver them. Returns the number of successful retries.
func (d *Daemon) retryPending() int {
	if d.Session == nil || d.Session.ErrorQueue == nil {
		return 0
	}

	ctx, retrySpan := platform.Tracer.Start(context.Background(), "daemon.retry_pending")
	defer retrySpan.End()

	maxRetries := d.opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}

	claimerID := fmt.Sprintf("daemon-%d", os.Getpid())
	entries, err := d.Session.ErrorQueue.ClaimPendingRetries(claimerID, maxRetries)
	if err != nil {
		d.logger.Error("Retry: claim pending: %v", err)
		return 0
	}

	if len(entries) == 0 {
		return 0
	}

	successCh := make(chan struct{}, len(entries))

	retryGroup := d.pool.NewGroup()
	for _, e := range entries {
		retryGroup.Submit(func() {
			originalPath := filepath.Join(e.SourceOutbox, e.OriginalName)
			result, deliverErr := DeliverData(ctx, originalPath, e.Data, d.opts.Routes, d.deliveryStore)
			if deliverErr != nil {
				if incErr := d.Session.ErrorQueue.IncrementRetry(e.Name, deliverErr.Error()); incErr != nil {
					d.logger.Warn("Retry: increment retry: %v", incErr)
				}
				if d.opts.Verbose {
					d.logger.Warn("Retry failed for %s (attempt %d): %v", e.OriginalName, e.RetryCount+1, deliverErr)
				}
				return
			}

			if markErr := d.Session.ErrorQueue.MarkResolved(e.Name); markErr != nil {
				d.logger.Warn("Retry: mark resolved: %v", markErr)
			}

			if d.dlog != nil {
				for _, target := range result.DeliveredTo {
					d.dlog.Retried(result.Kind, originalPath, target)
				}
			}
			if d.Session != nil {
				d.Session.RecordRetryEvent(e.OriginalName, result.Kind)
			}

			if d.opts.Verbose {
				d.logger.OK("Retry: delivered %s (kind=%s) to %v", e.OriginalName, result.Kind, result.DeliveredTo)
			}

			successCh <- struct{}{}
		})
	}
	retryGroup.Wait()
	close(successCh)

	successes := 0
	for range successCh {
		successes++
	}
	return successes
}

// ScanAndDeliver processes all existing .md files in the given outbox directory,
// delivering each one according to the provided routes. Files are delivered
// sequentially. Failed deliveries are enqueued via errorQueue (SQLite).
// If errorQueue is nil, failed files remain in the outbox for next startup.
func ScanAndDeliver(ctx context.Context, outboxDir string, routes []domain.ResolvedRoute, stateDir string, logger domain.Logger, ds domain.DeliveryStore, errorQueue domain.ErrorQueueStore) ([]*domain.DeliveryResult, []error) {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	entries, err := os.ReadDir(outboxDir)
	if err != nil {
		return nil, []error{fmt.Errorf("scan outbox %s: %w", outboxDir, err)}
	}

	// Filter eligible entries
	var filtered []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".phonewave-tmp-") {
			continue
		}
		filtered = append(filtered, entry)
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	// Deliver files sequentially. The caller (daemon startup scan) already
	// parallelises per-outbox via its worker pool, so a nested pool here
	// would multiply concurrency to NumCPU² and spike FD/memory usage.
	var results []*domain.DeliveryResult
	var errs []error
	for _, entry := range filtered {
		dmailPath := filepath.Join(outboxDir, entry.Name())

		data, readErr := os.ReadFile(dmailPath)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", dmailPath, readErr))
			continue
		}

		result, deliverErr := DeliverData(ctx, dmailPath, data, routes, ds)
		if deliverErr != nil {
			kind, _ := domain.ExtractDMailKind(data)
		if kind == "" { kind = "unknown" }
			meta := domain.ErrorMetadata{
				SourceOutbox: outboxDir,
				Kind:         kind,
				OriginalName: entry.Name(),
				Attempts:     1,
				Error:        deliverErr.Error(),
				Timestamp:    time.Now().UTC(),
			}
			if errorQueue == nil {
				logger.Error("Error queue unavailable, leaving %s in outbox", dmailPath)
			} else {
				name := fmt.Sprintf("%s-%s-%s", meta.Timestamp.Format("2006-01-02T150405.000000000"), meta.Kind, meta.OriginalName)
				if saveErr := errorQueue.Enqueue(name, data, meta); saveErr != nil {
					logger.Error("Error queue enqueue: %v", saveErr)
				} else {
					os.Remove(dmailPath)
				}
			}
			errs = append(errs, fmt.Errorf("deliver %s: %w", dmailPath, deliverErr))
			continue
		}

		results = append(results, result)
	}

	return results, errs
}
