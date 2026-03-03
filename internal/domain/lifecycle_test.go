package domain_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

// setupEcosystemDir creates a full 3-tool ecosystem in a temp directory.
//
// Layout:
//
//	.siren:      produces=specification, consumes=feedback
//	.expedition: produces=report,        consumes=specification,feedback
//	.gate: produces=feedback,      consumes=report
//
// Routes derived:
//
//	specification: .siren/outbox      → .expedition/inbox
//	report:        .expedition/outbox  → .gate/inbox
//	feedback:      .gate/outbox  → .siren/inbox, .expedition/inbox
func setupEcosystemDir(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	tools := []struct {
		dir      string
		produces []string
		consumes []string
	}{
		{".siren", []string{"specification"}, []string{"feedback"}},
		{".expedition", []string{"report"}, []string{"specification", "feedback"}},
		{".gate", []string{"feedback"}, []string{"report"}},
	}

	for _, tool := range tools {
		// Create outbox and inbox
		for _, sub := range []string{"outbox", "inbox"} {
			if err := os.MkdirAll(filepath.Join(repoDir, tool.dir, sub), 0755); err != nil {
				t.Fatal(err)
			}
		}

		// dmail-sendable SKILL.md (produces)
		if len(tool.produces) > 0 {
			dir := filepath.Join(repoDir, tool.dir, "skills", "dmail-sendable")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			b.WriteString("---\nname: dmail-sendable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n")
			for _, k := range tool.produces {
				b.WriteString("    - kind: " + k + "\n")
			}
			b.WriteString("---\n")
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(b.String()), 0644); err != nil {
				t.Fatal(err)
			}
		}

		// dmail-readable SKILL.md (consumes)
		if len(tool.consumes) > 0 {
			dir := filepath.Join(repoDir, tool.dir, "skills", "dmail-readable")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			b.WriteString("---\nname: dmail-readable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n")
			for _, k := range tool.consumes {
				b.WriteString("    - kind: " + k + "\n")
			}
			b.WriteString("---\n")
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(b.String()), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	return repoDir
}

// waitForFileExt polls until a file exists at path, or fails after timeout.
func waitForFileExt(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for file: %s", path)
		default:
			if _, err := os.Stat(path); err == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// waitForFileAbsent polls until a file no longer exists, or fails after timeout.
func waitForFileAbsent(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for file removal: %s", path)
		default:
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestLifecycle_FullEcosystem(t *testing.T) {
	// =====================================================================
	// Phase 1: Setup ecosystem + Init()
	// =====================================================================
	repoDir := setupEcosystemDir(t)

	result, err := session.Init([]string{repoDir})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if result.RepoCount != 1 {
		t.Fatalf("RepoCount = %d, want 1", result.RepoCount)
	}
	if len(result.Config.Repositories) != 1 {
		t.Fatalf("Repositories = %d, want 1", len(result.Config.Repositories))
	}
	// 3 endpoints * some routes
	if len(result.Config.Routes) < 3 {
		t.Fatalf("Routes = %d, want >= 3", len(result.Config.Routes))
	}
	// No orphans in a complete ecosystem
	if len(result.Orphans.UnconsumedKinds) != 0 {
		t.Errorf("unexpected unconsumed: %v", result.Orphans.UnconsumedKinds)
	}
	if len(result.Orphans.UnproducedKinds) != 0 {
		t.Errorf("unexpected unproduced: %v", result.Orphans.UnproducedKinds)
	}

	// Prepare daemon dependencies
	routes, err := session.ResolveRoutes(result.Config)
	if err != nil {
		t.Fatalf("ResolveRoutes: %v", err)
	}
	outboxDirs := session.CollectOutboxDirs(result.Config)
	if len(outboxDirs) < 3 {
		t.Fatalf("outboxDirs = %d, want >= 3", len(outboxDirs))
	}

	stateDir := filepath.Join(t.TempDir(), ".phonewave")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// =====================================================================
	// Phase 2: Place pre-existing D-Mail before daemon starts
	// =====================================================================
	preExistPath := filepath.Join(repoDir, ".siren", "outbox", "spec-preexist.md")
	if err := os.WriteFile(preExistPath, []byte(`---
dmail-schema-version: "1"
name: spec-preexist
kind: specification
description: "Pre-existing specification"
---

# Pre-existing
`), 0644); err != nil {
		t.Fatal(err)
	}

	// =====================================================================
	// Phase 3: Start daemon — startup scan should deliver pre-existing file
	// =====================================================================
	d, err := session.NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: outboxDirs,
		StateDir:   stateDir,
		Verbose:    true,
	}, domain.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	// Verify startup scan delivered pre-existing file
	expeditionInbox := filepath.Join(repoDir, ".expedition", "inbox")
	waitForFileExt(t, filepath.Join(expeditionInbox, "spec-preexist.md"), 5*time.Second)
	waitForFileAbsent(t, preExistPath, 5*time.Second)

	// =====================================================================
	// Phase 4: Runtime delivery via fsnotify — specification
	// =====================================================================
	runtimeSpec := filepath.Join(repoDir, ".siren", "outbox", "spec-runtime.md")
	if err := os.WriteFile(runtimeSpec, []byte(`---
dmail-schema-version: "1"
name: spec-runtime
kind: specification
description: "Runtime specification"
---

# Runtime Spec
`), 0644); err != nil {
		t.Fatal(err)
	}

	waitForFileExt(t, filepath.Join(expeditionInbox, "spec-runtime.md"), 5*time.Second)
	waitForFileAbsent(t, runtimeSpec, 5*time.Second)

	// =====================================================================
	// Phase 5: Multi-target delivery — feedback → siren + expedition
	// =====================================================================
	feedbackPath := filepath.Join(repoDir, ".gate", "outbox", "fb-lifecycle.md")
	if err := os.WriteFile(feedbackPath, []byte(`---
dmail-schema-version: "1"
name: fb-lifecycle
kind: feedback
description: "Lifecycle feedback"
---

# Feedback
`), 0644); err != nil {
		t.Fatal(err)
	}

	sirenInbox := filepath.Join(repoDir, ".siren", "inbox")
	waitForFileExt(t, filepath.Join(sirenInbox, "fb-lifecycle.md"), 5*time.Second)
	waitForFileExt(t, filepath.Join(expeditionInbox, "fb-lifecycle.md"), 5*time.Second)
	waitForFileAbsent(t, feedbackPath, 5*time.Second)

	// =====================================================================
	// Phase 6: Verify delivery log
	// =====================================================================
	logData, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read delivery log: %v", err)
	}
	logContent := string(logData)

	if !strings.Contains(logContent, "DELIVERED") {
		t.Error("delivery log missing DELIVERED entries")
	}
	if !strings.Contains(logContent, "REMOVED") {
		t.Error("delivery log missing REMOVED entries")
	}
	if !strings.Contains(logContent, "kind=specification") {
		t.Error("delivery log missing kind=specification")
	}
	if !strings.Contains(logContent, "kind=feedback") {
		t.Error("delivery log missing kind=feedback")
	}

	// Count DELIVERED lines — at least 4:
	//   spec-preexist → expedition (1) (startup scan)
	//   spec-runtime → expedition (1)
	//   fb-lifecycle → siren + expedition (2)
	deliveredCount := strings.Count(logContent, "DELIVERED")
	if deliveredCount < 4 {
		t.Errorf("DELIVERED count = %d, want >= 4", deliveredCount)
	}

	// =====================================================================
	// Phase 7: Malformed D-Mail — daemon must survive
	// =====================================================================
	malformedPath := filepath.Join(repoDir, ".siren", "outbox", "bad-mail.md")
	if err := os.WriteFile(malformedPath, []byte("This is not a valid D-Mail\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// Prove daemon still alive by delivering another valid D-Mail
	afterBadPath := filepath.Join(repoDir, ".siren", "outbox", "spec-after-bad.md")
	if err := os.WriteFile(afterBadPath, []byte(`---
dmail-schema-version: "1"
name: spec-after-bad
kind: specification
description: "After malformed"
---
`), 0644); err != nil {
		t.Fatal(err)
	}
	waitForFileExt(t, filepath.Join(expeditionInbox, "spec-after-bad.md"), 5*time.Second)

	// =====================================================================
	// Phase 8: Unknown kind — FAILED logged
	// =====================================================================
	unknownPath := filepath.Join(repoDir, ".siren", "outbox", "mystery.md")
	if err := os.WriteFile(unknownPath, []byte(`---
dmail-schema-version: "1"
name: mystery
kind: unknown_kind
description: "Unknown"
---
`), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	logData, err = os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read delivery log: %v", err)
	}
	if !strings.Contains(string(logData), "FAILED") {
		t.Error("delivery log missing FAILED entry for unknown kind")
	}

	// =====================================================================
	// Phase 9: Shutdown — PID file removed
	// =====================================================================
	pidPath := filepath.Join(stateDir, "watch.pid")
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file should exist while daemon is running")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("daemon error on shutdown: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should be removed after shutdown")
	}

	// =====================================================================
	// Phase 10: Restart daemon — delivery log persists (append-only)
	// =====================================================================
	logBeforeRestart, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read delivery log before restart: %v", err)
	}
	linesBefore := strings.Count(string(logBeforeRestart), "\n")

	d2, err := session.NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: outboxDirs,
		StateDir:   stateDir,
		Verbose:    true,
	}, domain.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon (restart): %v", err)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	errCh2 := make(chan error, 1)
	go func() { errCh2 <- d2.Run(ctx2) }()
	time.Sleep(200 * time.Millisecond)

	// Deliver one more D-Mail after restart
	restartPath := filepath.Join(repoDir, ".expedition", "outbox", "report-restart.md")
	if err := os.WriteFile(restartPath, []byte(`---
dmail-schema-version: "1"
name: report-restart
kind: report
description: "After restart"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	gateInbox := filepath.Join(repoDir, ".gate", "inbox")
	waitForFileExt(t, filepath.Join(gateInbox, "report-restart.md"), 5*time.Second)

	cancel2()
	if err := <-errCh2; err != nil {
		t.Errorf("daemon error on second shutdown: %v", err)
	}

	// Verify log grew (appended, not truncated)
	logAfterRestart, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read delivery log after restart: %v", err)
	}
	linesAfter := strings.Count(string(logAfterRestart), "\n")
	if linesAfter <= linesBefore {
		t.Errorf("delivery log should grow after restart: before=%d after=%d", linesBefore, linesAfter)
	}
}
