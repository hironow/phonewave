package phonewave

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveRoutes(t *testing.T) {
	// given — a config with one repo and routes using relative paths
	repoDir := t.TempDir()

	// Create outbox and inbox directories
	siren := filepath.Join(repoDir, ".siren")
	expedition := filepath.Join(repoDir, ".expedition")
	divergence := filepath.Join(repoDir, ".divergence")
	for _, dir := range []string{
		filepath.Join(siren, "outbox"),
		filepath.Join(siren, "inbox"),
		filepath.Join(expedition, "outbox"),
		filepath.Join(expedition, "inbox"),
		filepath.Join(divergence, "outbox"),
		filepath.Join(divergence, "inbox"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &Config{
		Repositories: []RepoConfig{
			{
				Path: repoDir,
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification", "feedback"}},
					{Dir: ".divergence", Produces: []string{"feedback"}, Consumes: []string{"report"}},
				},
			},
		},
		Routes: []RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
			{Kind: "report", From: ".expedition/outbox", To: []string{".divergence/inbox"}, Scope: "same_repository"},
			{Kind: "feedback", From: ".divergence/outbox", To: []string{".siren/inbox", ".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	// when
	resolved, err := ResolveRoutes(cfg)

	// then
	if err != nil {
		t.Fatalf("ResolveRoutes: %v", err)
	}
	if len(resolved) != 3 {
		t.Fatalf("resolved routes = %d, want 3", len(resolved))
	}

	// Verify absolute paths
	for _, r := range resolved {
		if !filepath.IsAbs(r.FromOutbox) {
			t.Errorf("FromOutbox %q is not absolute", r.FromOutbox)
		}
		for _, inbox := range r.ToInboxes {
			if !filepath.IsAbs(inbox) {
				t.Errorf("ToInbox %q is not absolute", inbox)
			}
		}
	}

	// Check specification route
	specRoute := findResolvedRoute(resolved, "specification")
	if specRoute == nil {
		t.Fatal("specification route not found")
	}
	if specRoute.FromOutbox != filepath.Join(repoDir, ".siren", "outbox") {
		t.Errorf("spec FromOutbox = %q, want %q", specRoute.FromOutbox, filepath.Join(repoDir, ".siren", "outbox"))
	}
	if len(specRoute.ToInboxes) != 1 || specRoute.ToInboxes[0] != filepath.Join(repoDir, ".expedition", "inbox") {
		t.Errorf("spec ToInboxes = %v", specRoute.ToInboxes)
	}

	// Check feedback route (multiple targets)
	fbRoute := findResolvedRoute(resolved, "feedback")
	if fbRoute == nil {
		t.Fatal("feedback route not found")
	}
	if len(fbRoute.ToInboxes) != 2 {
		t.Errorf("feedback ToInboxes = %d, want 2", len(fbRoute.ToInboxes))
	}
}

func TestResolveRoutes_MultiRepoSameEndpoint(t *testing.T) {
	// given — two repos, both containing .siren
	// Repo A: .siren produces "specification"
	// Repo B: .siren produces "alert"
	repoA := t.TempDir()
	repoB := t.TempDir()

	for _, dir := range []string{
		filepath.Join(repoA, ".siren", "outbox"),
		filepath.Join(repoA, ".expedition", "inbox"),
		filepath.Join(repoB, ".siren", "outbox"),
		filepath.Join(repoB, ".divergence", "inbox"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &Config{
		Repositories: []RepoConfig{
			{
				Path: repoA,
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: nil},
					{Dir: ".expedition", Produces: nil, Consumes: []string{"specification"}},
				},
			},
			{
				Path: repoB,
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"alert"}, Consumes: nil},
					{Dir: ".divergence", Produces: nil, Consumes: []string{"alert"}},
				},
			},
		},
	}
	cfg.UpdateRoutes()

	// when
	resolved, err := ResolveRoutes(cfg)

	// then
	if err != nil {
		t.Fatalf("ResolveRoutes: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved routes = %d, want 2", len(resolved))
	}

	// specification route must resolve to repo A paths
	specRoute := findResolvedRoute(resolved, "specification")
	if specRoute == nil {
		t.Fatal("specification route not found")
	}
	wantSpecFrom := filepath.Join(repoA, ".siren", "outbox")
	if specRoute.FromOutbox != wantSpecFrom {
		t.Errorf("specification FromOutbox = %q, want %q (repo A)", specRoute.FromOutbox, wantSpecFrom)
	}

	// alert route must resolve to repo B paths, NOT repo A
	alertRoute := findResolvedRoute(resolved, "alert")
	if alertRoute == nil {
		t.Fatal("alert route not found")
	}
	wantAlertFrom := filepath.Join(repoB, ".siren", "outbox")
	if alertRoute.FromOutbox != wantAlertFrom {
		t.Errorf("alert FromOutbox = %q, want %q (repo B)", alertRoute.FromOutbox, wantAlertFrom)
	}
	wantAlertTo := filepath.Join(repoB, ".divergence", "inbox")
	if len(alertRoute.ToInboxes) != 1 || alertRoute.ToInboxes[0] != wantAlertTo {
		t.Errorf("alert ToInboxes = %v, want [%s] (repo B)", alertRoute.ToInboxes, wantAlertTo)
	}
}

func findResolvedRoute(routes []ResolvedRoute, kind string) *ResolvedRoute {
	for i := range routes {
		if routes[i].Kind == kind {
			return &routes[i]
		}
	}
	return nil
}

func TestResolveRoutes_CollectsOutboxDirs(t *testing.T) {
	// given — same setup
	repoDir := t.TempDir()
	for _, dir := range []string{
		filepath.Join(repoDir, ".siren", "outbox"),
		filepath.Join(repoDir, ".expedition", "outbox"),
		filepath.Join(repoDir, ".divergence", "outbox"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &Config{
		Repositories: []RepoConfig{
			{
				Path: repoDir,
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
					{Dir: ".divergence", Produces: []string{"feedback"}, Consumes: []string{"report"}},
				},
			},
		},
		Routes: []RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	// when
	resolved, err := ResolveRoutes(cfg)

	// then
	if err != nil {
		t.Fatalf("ResolveRoutes: %v", err)
	}

	// OutboxDirs should contain all outbox directories from endpoints that produce
	outboxDirs := CollectOutboxDirs(cfg)
	if len(outboxDirs) != 3 {
		t.Errorf("outbox dirs = %d, want 3", len(outboxDirs))
	}
	_ = resolved
}

func TestDaemon_StartupScan(t *testing.T) {
	// given — a repo with a pre-existing file in outbox
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
name: spec-startup
kind: specification
description: "Pre-existing spec"
---

# Startup Test
`
	dmailPath := filepath.Join(outbox, "spec-startup.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when — scan existing outbox files
	results, errs := ScanAndDeliver(outbox, routes)

	// then
	if len(errs) != 0 {
		t.Fatalf("ScanAndDeliver errors: %v", errs)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Kind != "specification" {
		t.Errorf("kind = %q, want specification", results[0].Kind)
	}

	// File should be in inbox
	if _, err := os.Stat(filepath.Join(inbox, "spec-startup.md")); os.IsNotExist(err) {
		t.Error("D-Mail not found in inbox after startup scan")
	}

	// File should be removed from outbox
	if _, err := os.Stat(dmailPath); !os.IsNotExist(err) {
		t.Error("D-Mail should be removed from outbox after delivery")
	}
}

func TestDaemon_WatchAndDeliver(t *testing.T) {
	// given — a repo with outbox/inbox and a daemon watching
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		Verbose:    true,
	})
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	// Start daemon in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// when — write a D-Mail to outbox
	dmailContent := `---
name: spec-watch
kind: specification
description: "Watch test"
---

# Watch Test
`
	dmailPath := filepath.Join(outbox, "spec-watch.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for delivery (with timeout)
	deadline := time.After(5 * time.Second)
	delivered := false
	for !delivered {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for delivery")
		default:
			if _, err := os.Stat(filepath.Join(inbox, "spec-watch.md")); err == nil {
				delivered = true
			} else {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}

	// then — file should be in inbox
	if _, err := os.Stat(filepath.Join(inbox, "spec-watch.md")); os.IsNotExist(err) {
		t.Error("D-Mail not found in inbox")
	}

	// Source should be removed
	time.Sleep(100 * time.Millisecond) // allow removal to complete
	if _, err := os.Stat(dmailPath); !os.IsNotExist(err) {
		t.Error("D-Mail should be removed from outbox")
	}

	// Shutdown
	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("daemon error: %v", err)
	}
}

func TestDaemon_PIDFile(t *testing.T) {
	// given
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     []ResolvedRoute{},
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	})
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// then — PID file should exist
	pidPath := filepath.Join(stateDir, "watch.pid")
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file not created")
	}

	// Shutdown
	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("daemon error: %v", err)
	}

	// PID file should be removed after shutdown
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should be removed after shutdown")
	}
}
