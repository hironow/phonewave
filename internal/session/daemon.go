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
	phonewave "github.com/hironow/phonewave"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DaemonOptions configures the daemon behavior.
type DaemonOptions struct {
	Routes        []phonewave.ResolvedRoute
	OutboxDirs    []string
	StateDir      string
	ErrorStore    phonewave.ErrorStore
	Verbose       bool
	DryRun        bool
	RetryInterval time.Duration // 0 = disabled (default)
	MaxRetries    int           // default 10
}

// Daemon watches outbox directories and delivers D-Mails.
type Daemon struct {
	opts       DaemonOptions
	logger     *phonewave.Logger
	watcher    *fsnotify.Watcher
	dlog       *DeliveryLog
	errorStore phonewave.ErrorStore
	pool       pond.Pool
}

// NewDaemon creates a new Daemon with the given options and logger.
// If logger is nil, a silent logger (io.Discard) is used.
func NewDaemon(opts DaemonOptions, logger *phonewave.Logger) (*Daemon, error) {
	if logger == nil {
		logger = phonewave.NewLogger(nil, false)
	}
	watcher, err := fsnotify.NewWatcher() // nosemgrep: adr0005-fsnotify-watcher-without-close — stored in Daemon struct, closed in Run()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &Daemon{
		opts:       opts,
		logger:     logger,
		watcher:    watcher,
		errorStore: opts.ErrorStore,
		pool:       pond.NewPool(runtime.NumCPU()),
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
			scanCtx, scanSpan := phonewave.Tracer.Start(ctx, "daemon.startup_scan",
				trace.WithNewRoot(),
				trace.WithAttributes(attribute.String("outbox.dir", dir)),
			)
			results, errs := ScanAndDeliver(scanCtx, dir, d.opts.Routes, d.errorStore, d.logger)
			scanSpan.SetAttributes(attribute.Int("delivered.count", len(results)))
			scanSpan.End()
			for _, r := range results {
				if d.dlog != nil {
					for _, target := range r.DeliveredTo {
						d.dlog.Delivered(r.Kind, r.SourcePath, target)
					}
					d.dlog.Removed(r.SourcePath)
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

	// Optional retry ticker (nil channel disables the case)
	var retryCh <-chan time.Time
	if d.opts.RetryInterval > 0 {
		ticker := time.NewTicker(d.opts.RetryInterval)
		retryCh = ticker.C
		defer ticker.Stop()
	}

	// Event loop
	for {
		select {
		case <-ctx.Done():
			if d.opts.Verbose {
				d.logger.Info("Shutting down daemon")
			}
			return nil

		case event, ok := <-d.watcher.Events:
			if !ok {
				return nil
			}
			d.handleEvent(event)

		case err, ok := <-d.watcher.Errors:
			if !ok {
				return nil
			}
			d.logger.Warn("Watcher error: %v", err)

		case <-retryCh:
			d.retryPending()
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

	ctx, span := phonewave.Tracer.Start(context.Background(), "daemon.handle_event",
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

	result, err := DeliverData(ctx, event.Name, data, d.opts.Routes)
	if err != nil {
		kind := extractKindOrUnknown(data)
		d.logger.Error("Deliver %s: %v", event.Name, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if d.dlog != nil {
			d.dlog.Failed(kind, event.Name, err.Error())
		}

		if d.errorStore == nil {
			// No error store — leave file in outbox for retry on next restart
			return
		}

		now := time.Now().UTC()
		ts := now.Format("2006-01-02T150405.000000000")
		entryName := fmt.Sprintf("%s-%s-%s", ts, kind, filepath.Base(event.Name))
		entry := phonewave.RetryEntry{
			Name:         entryName,
			SourceOutbox: filepath.Dir(event.Name),
			Kind:         kind,
			OriginalName: filepath.Base(event.Name),
			Data:         data,
			Attempts:     1,
			Error:        err.Error(),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if saveErr := d.errorStore.RecordFailure(entry); saveErr != nil {
			d.logger.Error("Save to error queue: %v", saveErr)
			// Do NOT remove from outbox — startup scan will retry on next restart
			return
		}

		// Error store write succeeded — safe to remove from outbox
		os.Remove(event.Name)
		return
	}

	if d.dlog != nil {
		for _, target := range result.DeliveredTo {
			d.dlog.Delivered(result.Kind, result.SourcePath, target)
		}
		d.dlog.Removed(result.SourcePath)
	}

	if d.opts.Verbose {
		d.logger.OK("Delivered %s (kind=%s) to %v", result.SourcePath, result.Kind, result.DeliveredTo)
	}
}

// retryPending queries the error store for pending entries and attempts to
// re-deliver them. Eligible retries run concurrently via the daemon's worker pool.
func (d *Daemon) retryPending() {
	if d.errorStore == nil {
		return
	}

	ctx, retrySpan := phonewave.Tracer.Start(context.Background(), "daemon.retry_pending")
	defer retrySpan.End()

	maxRetries := d.opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}

	eligible, err := d.errorStore.ListPending(maxRetries)
	if err != nil {
		d.logger.Warn("Retry: list pending: %v", err)
		return
	}

	if len(eligible) == 0 {
		return
	}

	retryGroup := d.pool.NewGroup()
	for _, e := range eligible {
		retryGroup.Submit(func() {
			originalPath := filepath.Join(e.SourceOutbox, e.OriginalName)

			result, deliverErr := DeliverData(ctx, originalPath, e.Data, d.opts.Routes)
			if deliverErr != nil {
				if markErr := d.errorStore.MarkRetried(e.Name, deliverErr.Error()); markErr != nil {
					d.logger.Warn("Retry: update metadata: %v", markErr)
				}
				if d.opts.Verbose {
					d.logger.Warn("Retry failed for %s (attempt %d): %v", e.OriginalName, e.Attempts+1, deliverErr)
				}
				return
			}

			if removeErr := d.errorStore.RemoveEntry(e.Name); removeErr != nil {
				d.logger.Warn("Retry: remove error entry: %v", removeErr)
			}

			if d.dlog != nil {
				for _, target := range result.DeliveredTo {
					d.dlog.Retried(result.Kind, originalPath, target)
				}
			}

			if d.opts.Verbose {
				d.logger.OK("Retry: delivered %s (kind=%s) to %v", e.OriginalName, result.Kind, result.DeliveredTo)
			}
		})
	}
	retryGroup.Wait()
}

// extractKindOrUnknown attempts to extract the kind from D-Mail data,
// returning "unknown" if parsing fails.
func extractKindOrUnknown(data []byte) string {
	kind, err := phonewave.ExtractDMailKind(data)
	if err != nil {
		return "unknown"
	}
	return kind
}

// ResolveRoutes converts Config routes (relative paths) into ResolvedRoutes
// (absolute paths) that the delivery pipeline can use directly.
func ResolveRoutes(cfg *phonewave.Config) ([]phonewave.ResolvedRoute, error) {
	var resolved []phonewave.ResolvedRoute

	for _, route := range cfg.Routes {
		repoPath := route.RepoPath
		if repoPath == "" {
			// Fallback: derive repo from endpoint directory when RepoPath is unset
			repo, err := findRepoForRoute(cfg, route.From)
			if err != nil {
				return nil, err
			}
			repoPath = repo.Path
		}

		fromAbs := filepath.Join(repoPath, route.From)
		var toAbs []string
		for _, to := range route.To {
			toAbs = append(toAbs, filepath.Join(repoPath, to))
		}

		resolved = append(resolved, phonewave.ResolvedRoute{
			Kind:       route.Kind,
			FromOutbox: fromAbs,
			ToInboxes:  toAbs,
		})
	}

	return resolved, nil
}

// findRepoForRoute locates the repository that contains the given relative
// outbox path (e.g. ".siren/outbox").
func findRepoForRoute(cfg *phonewave.Config, fromPath string) (*phonewave.RepoConfig, error) {
	parts := strings.SplitN(fromPath, string(filepath.Separator), 2)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid from path: %q", fromPath)
	}
	dotDir := parts[0]

	for i := range cfg.Repositories {
		for _, ep := range cfg.Repositories[i].Endpoints {
			if ep.Dir == dotDir {
				return &cfg.Repositories[i], nil
			}
		}
	}
	return nil, fmt.Errorf("no repository found for route from %q", fromPath)
}

// CollectOutboxDirs returns all absolute outbox directory paths from endpoints
// that produce at least one kind. Consume-only endpoints are excluded because
// they may not have an outbox directory.
func CollectOutboxDirs(cfg *phonewave.Config) []string {
	var dirs []string
	for _, repo := range cfg.Repositories {
		for _, ep := range repo.Endpoints {
			if len(ep.Produces) > 0 {
				dirs = append(dirs, filepath.Join(repo.Path, ep.Dir, "outbox"))
			}
		}
	}
	return dirs
}

// ScanAndDeliver processes all existing .md files in the given outbox directory,
// delivering each one according to the provided routes. Files are delivered
// sequentially. Failed deliveries are saved to the error store (if provided).
func ScanAndDeliver(ctx context.Context, outboxDir string, routes []phonewave.ResolvedRoute, errStore phonewave.ErrorStore, logger *phonewave.Logger) ([]*phonewave.DeliveryResult, []error) {
	if logger == nil {
		logger = phonewave.NewLogger(nil, false)
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
	// would multiply concurrency to NumCPU^2 and spike FD/memory usage.
	var results []*phonewave.DeliveryResult
	var errs []error
	for _, entry := range filtered {
		dmailPath := filepath.Join(outboxDir, entry.Name())

		data, readErr := os.ReadFile(dmailPath)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", dmailPath, readErr))
			continue
		}

		result, deliverErr := DeliverData(ctx, dmailPath, data, routes)
		if deliverErr != nil {
			if errStore != nil {
				kind := extractKindOrUnknown(data)
				now := time.Now().UTC()
				ts := now.Format("2006-01-02T150405.000000000")
				entryName := fmt.Sprintf("%s-%s-%s", ts, kind, entry.Name())
				retryEntry := phonewave.RetryEntry{
					Name:         entryName,
					SourceOutbox: outboxDir,
					Kind:         kind,
					OriginalName: entry.Name(),
					Data:         data,
					Attempts:     1,
					Error:        deliverErr.Error(),
					CreatedAt:    now,
					UpdatedAt:    now,
				}
				if saveErr := errStore.RecordFailure(retryEntry); saveErr != nil {
					logger.Error("Save to error queue: %v", saveErr)
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
