package phonewave

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRetryBackoff_InitialInterval(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 60*time.Second)

	// when
	d := b.Next()

	// then: should be within ±25% of base (1s)
	if d < 750*time.Millisecond || d > 1250*time.Millisecond {
		t.Errorf("initial interval: got %v, want ~1s (±25%%)", d)
	}
}

func TestRetryBackoff_ExponentialIncrease(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 60*time.Second)

	// when: record 3 consecutive failures
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()

	// then: base should be 8s (1s * 2^3) — Next with jitter should be ~6-10s
	d := b.Next()
	if d < 6*time.Second || d > 10*time.Second {
		t.Errorf("after 3 failures: got %v, want ~8s (±25%%)", d)
	}
}

func TestRetryBackoff_CappedAtMax(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 10*time.Second)

	// when: record many failures (should cap at max)
	for range 20 {
		b.RecordFailure()
	}

	// then: should be within ±25% of max (10s), never exceed 12.5s
	d := b.Next()
	if d > 12500*time.Millisecond {
		t.Errorf("capped interval: got %v, should not exceed 12.5s (max 10s + 25%% jitter)", d)
	}
	if d < 7500*time.Millisecond {
		t.Errorf("capped interval: got %v, should be at least 7.5s (max 10s - 25%% jitter)", d)
	}
}

func TestRetryBackoff_ResetOnSuccess(t *testing.T) {
	// given
	b := NewRetryBackoff(1*time.Second, 60*time.Second)
	b.RecordFailure()
	b.RecordFailure()
	b.RecordFailure()

	// when: record success
	b.RecordSuccess()

	// then: should be back to base interval (~1s)
	d := b.Next()
	if d < 750*time.Millisecond || d > 1250*time.Millisecond {
		t.Errorf("after reset: got %v, want ~1s (±25%%)", d)
	}
}

func TestRetryBackoff_ConsecutiveFailures(t *testing.T) {
	// given
	b := NewRetryBackoff(100*time.Millisecond, 10*time.Second)

	// when/then: each failure should roughly double the interval
	b.RecordFailure() // current = 200ms
	d1 := b.Next()

	b.RecordFailure() // current = 400ms
	d2 := b.Next()

	// d2 should be roughly 2x d1 (within jitter bounds)
	if d2 < d1 {
		t.Errorf("second failure interval %v should be > first %v", d2, d1)
	}
}

// waitForFile polls until a file exists at path, or fails after timeout.
func waitForFile(t *testing.T, path string, timeout time.Duration) {
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
dmail-schema-version: "1"
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
	results, errs := ScanAndDeliver(context.Background(), outbox, routes, stateDir, NewLogger(io.Discard, false))

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
	}, NewLogger(io.Discard, false))
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
dmail-schema-version: "1"
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

func TestDaemon_StartupScan_LogsToDeliveryLog(t *testing.T) {
	// given — a repo with a pre-existing file in outbox before daemon starts
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailPath := filepath.Join(outbox, "spec-log-test.md")
	if err := os.WriteFile(dmailPath, []byte(`---
dmail-schema-version: "1"
name: spec-log-test
kind: specification
description: "Startup log test"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		Verbose:    true,
	}, NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	// when — start daemon (startup scan should deliver the file)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	// Wait for startup scan to deliver
	waitForFile(t, filepath.Join(inbox, "spec-log-test.md"), 5*time.Second)

	// then — delivery.log should contain DELIVERED and REMOVED entries from startup scan
	logData, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read delivery log: %v", err)
	}
	logContent := string(logData)

	if !strings.Contains(logContent, "DELIVERED") {
		t.Error("delivery log missing DELIVERED entry from startup scan")
	}
	if !strings.Contains(logContent, "kind=specification") {
		t.Error("delivery log missing kind=specification from startup scan")
	}
	if !strings.Contains(logContent, "REMOVED") {
		t.Error("delivery log missing REMOVED entry from startup scan")
	}

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
	}, NewLogger(io.Discard, false))
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

func TestDaemon_ConcurrentBurstDelivery(t *testing.T) {
	// given — daemon watching an outbox, burst of 5 files written rapidly
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
	}, NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)

	// when — write 5 files in rapid succession (burst)
	const burstCount = 5
	for i := range burstCount {
		content := strings.NewReader("---\ndmail-schema-version: \"1\"\nname: burst-" + strconv.Itoa(i) + "\nkind: specification\ndescription: \"burst\"\n---\n\n# Burst\n")
		data, _ := io.ReadAll(content)
		name := "burst-" + strconv.Itoa(i) + ".md"
		if err := os.WriteFile(filepath.Join(outbox, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// then — all 5 files should be delivered to inbox within timeout
	for i := range burstCount {
		name := "burst-" + strconv.Itoa(i) + ".md"
		waitForFile(t, filepath.Join(inbox, name), 10*time.Second)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("daemon error: %v", err)
	}
}
