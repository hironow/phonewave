package phonewave

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DaemonOptions configures the daemon behavior.
type DaemonOptions struct {
	Routes        []ResolvedRoute
	OutboxDirs    []string
	StateDir      string
	Verbose       bool
	DryRun        bool
	RetryInterval time.Duration // 0 = disabled (default)
	MaxRetries    int           // default 10
}

// Daemon watches outbox directories and delivers D-Mails.
type Daemon struct {
	opts    DaemonOptions
	watcher *fsnotify.Watcher
	dlog    *DeliveryLog
}

// NewDaemon creates a new Daemon with the given options.
func NewDaemon(opts DaemonOptions) (*Daemon, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &Daemon{
		opts:    opts,
		watcher: watcher,
	}, nil
}

// Run starts the daemon event loop. It blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
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
			LogInfo("Watching %s", dir)
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

	// Startup scan: deliver any files that accumulated while daemon was down
	for _, dir := range d.opts.OutboxDirs {
		results, errs := ScanAndDeliver(dir, d.opts.Routes, d.opts.StateDir)
		for _, r := range results {
			if d.opts.Verbose {
				LogOK("Startup: delivered %s (kind=%s) to %v", r.SourcePath, r.Kind, r.DeliveredTo)
			}
		}
		for _, err := range errs {
			LogWarn("Startup scan: %v", err)
		}
	}

	if d.opts.Verbose {
		LogOK("Daemon started (PID %d), watching %d outbox directories", os.Getpid(), len(d.opts.OutboxDirs))
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
				LogInfo("Shutting down daemon")
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
			LogWarn("Watcher error: %v", err)

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

	// Small delay to let the file be fully written
	time.Sleep(50 * time.Millisecond)

	if d.opts.DryRun {
		LogInfo("[dry-run] Detected %s", event.Name)
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
		LogError("Read %s: %v", event.Name, readErr)
		return
	}

	result, err := DeliverData(event.Name, data, d.opts.Routes)
	if err != nil {
		kind := extractKindOrUnknown(data)
		LogError("Deliver %s: %v", event.Name, err)
		if d.dlog != nil {
			d.dlog.Failed(kind, event.Name, err.Error())
		}

		meta := ErrorMetadata{
			SourceOutbox: filepath.Dir(event.Name),
			Kind:         kind,
			OriginalName: filepath.Base(event.Name),
			Attempts:     1,
			Error:        err.Error(),
			Timestamp:    time.Now().UTC(),
		}
		if saveErr := SaveToErrorQueue(d.opts.StateDir, meta, data); saveErr != nil {
			LogError("Save to error queue: %v", saveErr)
			// Do NOT remove from outbox — startup scan will retry on next restart
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
		d.dlog.Removed(result.SourcePath)
	}

	if d.opts.Verbose {
		LogOK("Delivered %s (kind=%s) to %v", result.SourcePath, result.Kind, result.DeliveredTo)
	}
}

// retryPending scans the error queue and attempts to re-deliver entries
// that have not exceeded MaxRetries.
func (d *Daemon) retryPending() {
	errorsDir := filepath.Join(d.opts.StateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		return // no error queue directory yet
	}

	maxRetries := d.opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".err") {
			continue
		}

		sidecarPath := filepath.Join(errorsDir, entry.Name())
		meta, err := LoadErrorMetadata(sidecarPath)
		if err != nil {
			LogWarn("Retry: load metadata %s: %v", sidecarPath, err)
			continue
		}

		if meta.Attempts >= maxRetries {
			continue
		}

		// Read the D-Mail data file (sidecar path minus ".err")
		dmailPath := strings.TrimSuffix(sidecarPath, ".err")
		data, err := os.ReadFile(dmailPath)
		if err != nil {
			LogWarn("Retry: read %s: %v", dmailPath, err)
			continue
		}

		// Reconstruct the original outbox path for DeliverData
		originalPath := filepath.Join(meta.SourceOutbox, meta.OriginalName)

		result, deliverErr := DeliverData(originalPath, data, d.opts.Routes)
		if deliverErr != nil {
			if err := UpdateErrorMetadata(sidecarPath, deliverErr.Error()); err != nil {
				LogWarn("Retry: update metadata: %v", err)
			}
			if d.opts.Verbose {
				LogWarn("Retry failed for %s (attempt %d): %v", meta.OriginalName, meta.Attempts+1, deliverErr)
			}
			continue
		}

		// Success — remove from error queue and log
		if err := RemoveErrorEntry(dmailPath); err != nil {
			LogWarn("Retry: remove error entry: %v", err)
		}

		if d.dlog != nil {
			for _, target := range result.DeliveredTo {
				d.dlog.Retried(result.Kind, originalPath, target)
			}
		}

		if d.opts.Verbose {
			LogOK("Retry: delivered %s (kind=%s) to %v", meta.OriginalName, result.Kind, result.DeliveredTo)
		}
	}
}

// extractKindOrUnknown attempts to extract the kind from D-Mail data,
// returning "unknown" if parsing fails.
func extractKindOrUnknown(data []byte) string {
	kind, err := ExtractDMailKind(data)
	if err != nil {
		return "unknown"
	}
	return kind
}

// ResolveRoutes converts Config routes (relative paths) into ResolvedRoutes
// (absolute paths) that the delivery pipeline can use directly.
func ResolveRoutes(cfg *Config) ([]ResolvedRoute, error) {
	var resolved []ResolvedRoute

	for _, route := range cfg.Routes {
		repoPath := route.RepoPath
		if repoPath == "" {
			// Fallback for legacy configs without RepoPath
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

		resolved = append(resolved, ResolvedRoute{
			Kind:       route.Kind,
			FromOutbox: fromAbs,
			ToInboxes:  toAbs,
		})
	}

	return resolved, nil
}

// findRepoForRoute locates the repository that contains the given relative
// outbox path (e.g. ".siren/outbox").
func findRepoForRoute(cfg *Config, fromPath string) (*RepoConfig, error) {
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
func CollectOutboxDirs(cfg *Config) []string {
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
// delivering each one according to the provided routes. Failed deliveries are
// saved to the error queue in stateDir.
func ScanAndDeliver(outboxDir string, routes []ResolvedRoute, stateDir string) ([]*DeliveryResult, []error) {
	entries, err := os.ReadDir(outboxDir)
	if err != nil {
		return nil, []error{fmt.Errorf("scan outbox %s: %w", outboxDir, err)}
	}

	var results []*DeliveryResult
	var errs []error

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

		dmailPath := filepath.Join(outboxDir, entry.Name())

		data, readErr := os.ReadFile(dmailPath)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", dmailPath, readErr))
			continue
		}

		result, deliverErr := DeliverData(dmailPath, data, routes)
		if deliverErr != nil {
			errs = append(errs, fmt.Errorf("deliver %s: %w", dmailPath, deliverErr))

			kind := extractKindOrUnknown(data)
			meta := ErrorMetadata{
				SourceOutbox: outboxDir,
				Kind:         kind,
				OriginalName: entry.Name(),
				Attempts:     1,
				Error:        deliverErr.Error(),
				Timestamp:    time.Now().UTC(),
			}
			if saveErr := SaveToErrorQueue(stateDir, meta, data); saveErr != nil {
				LogError("Save to error queue: %v", saveErr)
				// Do NOT remove from outbox — preserve for future retry
				continue
			}
			os.Remove(dmailPath)
			continue
		}
		results = append(results, result)
	}

	return results, errs
}
