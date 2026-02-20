package phonewave

import (
	"context"
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
	Routes     []ResolvedRoute
	OutboxDirs []string
	StateDir   string
	Verbose    bool
	DryRun     bool
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

	// Startup scan: deliver any files that accumulated while daemon was down
	for _, dir := range d.opts.OutboxDirs {
		results, errs := ScanAndDeliver(dir, d.opts.Routes)
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
		}
	}
}

// handleEvent processes a single fsnotify event.
func (d *Daemon) handleEvent(event fsnotify.Event) {
	// Only react to Create events for .md files
	if !event.Has(fsnotify.Create) {
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

	result, err := Deliver(event.Name, d.opts.Routes)
	if err != nil {
		LogError("Deliver %s: %v", event.Name, err)
		if d.dlog != nil {
			d.dlog.Failed("unknown", event.Name, err.Error())
		}
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

// ResolveRoutes converts Config routes (relative paths) into ResolvedRoutes
// (absolute paths) that the delivery pipeline can use directly.
func ResolveRoutes(cfg *Config) ([]ResolvedRoute, error) {
	var resolved []ResolvedRoute

	for _, route := range cfg.Routes {
		repo, err := findRepoForRoute(cfg, route.From)
		if err != nil {
			return nil, err
		}

		fromAbs := filepath.Join(repo.Path, route.From)
		var toAbs []string
		for _, to := range route.To {
			toAbs = append(toAbs, filepath.Join(repo.Path, to))
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
// that have at least one produces or consumes declaration.
func CollectOutboxDirs(cfg *Config) []string {
	var dirs []string
	for _, repo := range cfg.Repositories {
		for _, ep := range repo.Endpoints {
			if len(ep.Produces) > 0 || len(ep.Consumes) > 0 {
				dirs = append(dirs, filepath.Join(repo.Path, ep.Dir, "outbox"))
			}
		}
	}
	return dirs
}

// ScanAndDeliver processes all existing .md files in the given outbox directory,
// delivering each one according to the provided routes.
func ScanAndDeliver(outboxDir string, routes []ResolvedRoute) ([]*DeliveryResult, []error) {
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
		result, err := Deliver(dmailPath, routes)
		if err != nil {
			errs = append(errs, fmt.Errorf("deliver %s: %w", dmailPath, err))
			continue
		}
		results = append(results, result)
	}

	return results, errs
}
