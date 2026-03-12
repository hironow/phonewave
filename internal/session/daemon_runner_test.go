package session

// white-box-reason: session internals: tests unexported daemon adapter wiring and state construction

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
)

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

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	// when — scan existing outbox files
	results, errs := ScanAndDeliver(context.Background(), outbox, routes, stateDir, platform.NewLogger(io.Discard, false), ds, nil, nil)

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
	if _, err := os.Stat(filepath.Join(inbox, "spec-startup.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("D-Mail not found in inbox after startup scan")
	}

	// File should be removed from outbox
	if _, err := os.Stat(dmailPath); !errors.Is(err, fs.ErrNotExist) {
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

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		Verbose:    true,
	}, platform.NewLogger(io.Discard, false))
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
	waitForFile(t, filepath.Join(inbox, "spec-watch.md"), 5*time.Second)

	// then — file should be in inbox
	if _, err := os.Stat(filepath.Join(inbox, "spec-watch.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("D-Mail not found in inbox")
	}

	// Source should be removed
	time.Sleep(100 * time.Millisecond) // allow removal to complete
	if _, err := os.Stat(dmailPath); !errors.Is(err, fs.ErrNotExist) {
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

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		Verbose:    true,
	}, platform.NewLogger(io.Discard, false))
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

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     []domain.ResolvedRoute{},
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
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
	if _, err := os.Stat(pidPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("PID file not created")
	}

	// Shutdown
	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("daemon error: %v", err)
	}

	// PID file should be removed after shutdown
	if _, err := os.Stat(pidPath); !errors.Is(err, fs.ErrNotExist) {
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

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		Verbose:    true,
	}, platform.NewLogger(io.Discard, false))
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

// --- Edge Case tests ---

func TestDaemon_MalformedDMail(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		Verbose:    true,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// when — write a malformed D-Mail (no frontmatter)
	malformed := []byte("This is not a valid D-Mail\nNo frontmatter here\n")
	if err := os.WriteFile(filepath.Join(outbox, "bad-file.md"), malformed, 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	// then — daemon should NOT crash, inbox should be empty
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("inbox should be empty after malformed D-Mail, got %d files", len(entries))
	}

	// Daemon should still be running — test by sending a valid file
	validContent := `---
dmail-schema-version: "1"
name: spec-after-bad
kind: specification
description: "Valid after malformed"
---

# Valid
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-after-bad.md"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	waitForFile(t, filepath.Join(inbox, "spec-after-bad.md"), 5*time.Second)

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("daemon error: %v", err)
	}
}

func TestDaemon_UnknownKind(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	errorQueue, eqErr := NewSQLiteErrorQueueStore(stateDir)
	if eqErr != nil {
		t.Fatalf("create error queue store: %v", eqErr)
	}
	t.Cleanup(func() { errorQueue.Close() })

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.session = &DaemonSession{ErrorQueue: errorQueue}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	unknownContent := `---
name: mystery-001
kind: mystery
description: "Unknown kind"
---

# Mystery
`
	if err := os.WriteFile(filepath.Join(outbox, "mystery-001.md"), []byte(unknownContent), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("inbox should be empty for unknown kind, got %d files", len(entries))
	}

	if _, err := os.Stat(filepath.Join(outbox, "mystery-001.md")); err == nil {
		t.Error("source should be removed from outbox on delivery failure (moved to error queue)")
	}

	// Verify error is in SQLite error queue
	pendingCount, countErr := errorQueue.PendingCount(10)
	if countErr != nil {
		t.Fatalf("PendingCount: %v", countErr)
	}
	if pendingCount != 1 {
		t.Errorf("SQLite error queue pending count = %d, want 1", pendingCount)
	}

	cancel()
	<-errCh
}

func TestDaemon_IgnoresNonMdFiles(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	for _, name := range []string{"notes.txt", "data.json", ".DS_Store", "README"} {
		if err := os.WriteFile(filepath.Join(outbox, name), []byte("not a dmail"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(300 * time.Millisecond)

	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("inbox should be empty, non-.md files should be ignored, got %d", len(entries))
	}

	cancel()
	<-errCh
}

func TestScanAndDeliver_IgnoresTempFiles(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(outbox, ".phonewave-tmp-12345"), []byte("temp"), 0644); err != nil {
		t.Fatal(err)
	}

	validContent := `---
dmail-schema-version: "1"
name: spec-valid
kind: specification
description: "Valid"
---
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-valid.md"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	results, errs := ScanAndDeliver(context.Background(), outbox, routes, stateDir, platform.NewLogger(io.Discard, false), ds, nil, nil)

	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 (temp file should be skipped)", len(results))
	}
	if results[0].Kind != "specification" {
		t.Errorf("kind = %q, want specification", results[0].Kind)
	}

	if _, err := os.Stat(filepath.Join(outbox, ".phonewave-tmp-12345")); errors.Is(err, fs.ErrNotExist) {
		t.Error("temp file should not be removed by ScanAndDeliver")
	}
}

func TestScanAndDeliver_MixedValidInvalid(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(outbox, "spec-001.md"), []byte(`---
dmail-schema-version: "1"
name: spec-001
kind: specification
description: "Valid"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(outbox, "bad-002.md"), []byte("no frontmatter here"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(outbox, "spec-003.md"), []byte(`---
dmail-schema-version: "1"
name: spec-003
kind: specification
description: "Also valid"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)
	errorQueue, eqErr := NewSQLiteErrorQueueStore(stateDir)
	if eqErr != nil {
		t.Fatalf("create error queue store: %v", eqErr)
	}
	t.Cleanup(func() { errorQueue.Close() })

	results, errs := ScanAndDeliver(context.Background(), outbox, routes, stateDir, platform.NewLogger(io.Discard, false), ds, errorQueue, nil)

	if len(results) != 2 {
		t.Errorf("results = %d, want 2 (valid D-Mails delivered)", len(results))
	}
	if len(errs) != 1 {
		t.Errorf("errors = %d, want 1 (one invalid file)", len(errs))
	}

	if _, err := os.Stat(filepath.Join(inbox, "spec-001.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("spec-001.md should be in inbox")
	}
	if _, err := os.Stat(filepath.Join(inbox, "spec-003.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("spec-003.md should be in inbox")
	}

	if _, err := os.Stat(filepath.Join(outbox, "bad-002.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("bad-002.md should be removed from outbox (moved to error queue)")
	}

	// Verify error is in SQLite error queue
	pendingCount, countErr := errorQueue.PendingCount(10)
	if countErr != nil {
		t.Fatalf("PendingCount: %v", countErr)
	}
	if pendingCount != 1 {
		t.Errorf("SQLite error queue pending count = %d, want 1", pendingCount)
	}
}

func TestScanAndDeliver_EmptyOutbox(t *testing.T) {
	outbox := t.TempDir()
	stateDir := t.TempDir()
	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{"/tmp/nope"}},
	}

	ds := newTestDeliveryStore(t)

	results, errs := ScanAndDeliver(context.Background(), outbox, routes, stateDir, platform.NewLogger(io.Discard, false), ds, nil, nil)

	if len(results) != 0 {
		t.Errorf("results = %d, want 0", len(results))
	}
	if len(errs) != 0 {
		t.Errorf("errors = %v, want none", errs)
	}
}

func TestDaemon_MultipleOutboxes(t *testing.T) {
	repoDir := t.TempDir()
	outbox1 := filepath.Join(repoDir, ".siren", "outbox")
	outbox2 := filepath.Join(repoDir, ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, ".expedition", "inbox")
	inbox2 := filepath.Join(repoDir, ".siren", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox1, outbox2, inbox1, inbox2, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox1, ToInboxes: []string{inbox1}},
		{Kind: "design-feedback", FromOutbox: outbox2, ToInboxes: []string{inbox2}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox1, outbox2},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	specContent := `---
dmail-schema-version: "1"
name: spec-multi
kind: specification
description: "Multi outbox test"
---
`
	fbContent := `---
dmail-schema-version: "1"
name: fb-multi
kind: design-feedback
description: "Multi outbox test"
---
`
	if err := os.WriteFile(filepath.Join(outbox1, "spec-multi.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outbox2, "fb-multi.md"), []byte(fbContent), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for multi-outbox delivery")
		default:
			_, err1 := os.Stat(filepath.Join(inbox1, "spec-multi.md"))
			_, err2 := os.Stat(filepath.Join(inbox2, "fb-multi.md"))
			if err1 == nil && err2 == nil {
				goto bothDelivered
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
bothDelivered:

	if _, err := os.Stat(filepath.Join(inbox1, "spec-multi.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("spec-multi.md not found in inbox1")
	}
	if _, err := os.Stat(filepath.Join(inbox2, "fb-multi.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("fb-multi.md not found in inbox2")
	}

	cancel()
	<-errCh
}

func TestDaemon_BurstDelivery(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	for i := range 5 {
		content := []byte(`---
dmail-schema-version: "1"
name: spec-burst-` + string(rune('0'+i)) + `
kind: specification
description: "Burst test"
---
`)
		name := "spec-burst-" + string(rune('0'+i)) + ".md"
		if err := os.WriteFile(filepath.Join(outbox, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			entries, _ := os.ReadDir(inbox)
			t.Fatalf("timeout: only %d/5 files delivered to inbox", len(entries))
		default:
			entries, _ := os.ReadDir(inbox)
			if len(entries) >= 5 {
				goto allDelivered
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
allDelivered:

	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Errorf("inbox has %d files, want 5", len(entries))
	}

	outboxEntries, _ := os.ReadDir(outbox)
	mdCount := 0
	for _, e := range outboxEntries {
		if filepath.Ext(e.Name()) == ".md" {
			mdCount++
		}
	}
	if mdCount != 0 {
		t.Errorf("outbox still has %d .md files, want 0", mdCount)
	}

	cancel()
	<-errCh
}

func TestDaemon_PreservesOutboxFileWhenErrorQueueFails(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// No ErrorQueue injected (Session is nil) — handleEvent leaves file in outbox
	routes := []domain.ResolvedRoute{}

	dmailContent := `---
dmail-schema-version: "1"
name: spec-preserve
kind: specification
description: "Preserve test"
---
`
	dmailPath := filepath.Join(outbox, "spec-preserve.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()
	d.deliveryStore = newTestDeliveryStore(t)

	d.handleEvent(fsnotify.Event{
		Name: dmailPath,
		Op:   fsnotify.Create,
	})

	if _, err := os.Stat(dmailPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("outbox file was deleted even though error queue write failed — D-Mail lost permanently")
	}
}

func TestScanAndDeliver_PreservesOutboxFileWhenErrorQueueFails(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	dmailContent := `---
dmail-schema-version: "1"
name: spec-scan-preserve
kind: specification
description: "Preserve test"
---
`
	dmailPath := filepath.Join(outbox, "spec-scan-preserve.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{}
	ds := newTestDeliveryStore(t)

	// nil errorQueue — file should be preserved in outbox
	ScanAndDeliver(context.Background(), outbox, routes, stateDir, platform.NewLogger(io.Discard, false), ds, nil, nil)

	if _, err := os.Stat(dmailPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("outbox file was deleted even though error queue write failed — D-Mail lost permanently")
	}
}

func TestDaemon_HandleRenameEvent(t *testing.T) {
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
name: spec-rename
kind: specification
description: "Rename event test"
---

# Rename Test
`
	dmailPath := filepath.Join(outbox, "spec-rename.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()
	d.deliveryStore = newTestDeliveryStore(t)

	d.handleEvent(fsnotify.Event{
		Name: dmailPath,
		Op:   fsnotify.Rename,
	})

	if _, err := os.Stat(filepath.Join(inbox, "spec-rename.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("D-Mail not delivered to inbox on Rename event")
	}
}

func TestDaemon_HandleRenameEvent_FileGone(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:     []domain.ResolvedRoute{},
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()
	d.deliveryStore = newTestDeliveryStore(t)

	d.handleEvent(fsnotify.Event{
		Name: filepath.Join(outbox, "gone.md"),
		Op:   fsnotify.Rename,
	})
}

func TestDaemon_RetrySucceeds(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	errorQueue, eqErr := NewSQLiteErrorQueueStore(stateDir)
	if eqErr != nil {
		t.Fatalf("create error queue store: %v", eqErr)
	}
	t.Cleanup(func() { errorQueue.Close() })

	dmailData := []byte("---\ndmail-schema-version: \"1\"\nname: spec-retry\nkind: specification\ndescription: \"Retry test\"\n---\n\n# Retry Test\n")
	meta := domain.ErrorMetadata{
		SourceOutbox: outbox,
		Kind:         "specification",
		OriginalName: "spec-retry.md",
		Attempts:     1,
		Error:        "no route for kind",
		Timestamp:    time.Now().UTC(),
	}
	if err := errorQueue.Enqueue("spec-retry-err", dmailData, meta); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	dlog, dlErr := NewDeliveryLog(stateDir)
	if dlErr != nil {
		t.Fatal(dlErr)
	}
	defer dlog.Close()

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:        routes,
		OutboxDirs:    []string{outbox},
		StateDir:      stateDir,
		Verbose:       true,
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    10,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.session = &DaemonSession{ErrorQueue: errorQueue}
	d.dlog = dlog
	d.deliveryStore = newTestDeliveryStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	waitForFile(t, filepath.Join(inbox, "spec-retry.md"), 3*time.Second)

	if _, err := os.Stat(filepath.Join(inbox, "spec-retry.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("D-Mail not found in inbox after retry")
	}

	// Verify entry marked as resolved in SQLite
	pendingCount, countErr := errorQueue.PendingCount(10)
	if countErr != nil {
		t.Fatalf("PendingCount: %v", countErr)
	}
	if pendingCount != 0 {
		t.Errorf("SQLite error queue pending count = %d, want 0", pendingCount)
	}

	logData, _ := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if !strings.Contains(string(logData), "RETRIED") {
		t.Error("delivery log should contain RETRIED entry")
	}

	cancel()
	<-errCh
}

func TestDaemon_RetryExceedsMaxAttempts(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	errorQueue, eqErr := NewSQLiteErrorQueueStore(stateDir)
	if eqErr != nil {
		t.Fatalf("create error queue store: %v", eqErr)
	}
	t.Cleanup(func() { errorQueue.Close() })

	dmailData := []byte("---\ndmail-schema-version: \"1\"\nname: spec-maxed\nkind: specification\ndescription: \"Max retry\"\n---\n")
	meta := domain.ErrorMetadata{
		SourceOutbox: outbox,
		Kind:         "specification",
		OriginalName: "spec-maxed.md",
		Attempts:     10,
		Error:        "no route for kind",
		Timestamp:    time.Now().UTC(),
	}
	if err := errorQueue.Enqueue("spec-maxed-err", dmailData, meta); err != nil {
		t.Fatal(err)
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:        []domain.ResolvedRoute{},
		OutboxDirs:    []string{outbox},
		StateDir:      stateDir,
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    10,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.session = &DaemonSession{ErrorQueue: errorQueue}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	time.Sleep(350 * time.Millisecond)

	// Verify entry is still pending but not claimed (retry_count >= maxRetries)
	pendingCount, countErr := errorQueue.PendingCount(10)
	if countErr != nil {
		t.Fatalf("PendingCount: %v", countErr)
	}
	if pendingCount != 0 {
		t.Errorf("pending count = %d, want 0 (Attempts=10 >= MaxRetries=10, should not be claimable)", pendingCount)
	}

	cancel()
	<-errCh
}

func TestDaemon_RetryDisabledWhenZeroInterval(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	errorQueue, eqErr := NewSQLiteErrorQueueStore(stateDir)
	if eqErr != nil {
		t.Fatalf("create error queue store: %v", eqErr)
	}
	t.Cleanup(func() { errorQueue.Close() })

	dmailData := []byte("---\ndmail-schema-version: \"1\"\nname: spec-nope\nkind: specification\ndescription: \"No retry\"\n---\n")
	meta := domain.ErrorMetadata{
		SourceOutbox: outbox,
		Kind:         "specification",
		OriginalName: "spec-nope.md",
		Attempts:     1,
		Error:        "no route for kind",
		Timestamp:    time.Now().UTC(),
	}
	if err := errorQueue.Enqueue("spec-nope-err", dmailData, meta); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(domain.DaemonOptions{
		Routes:        routes,
		OutboxDirs:    []string{outbox},
		StateDir:      stateDir,
		RetryInterval: 0,
	}, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.session = &DaemonSession{ErrorQueue: errorQueue}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	time.Sleep(300 * time.Millisecond)

	if _, err := os.Stat(filepath.Join(inbox, "spec-nope.md")); err == nil {
		t.Error("D-Mail should NOT be in inbox (retry disabled)")
	}

	// Entry should still be pending in SQLite (retry not attempted)
	pendingCount, countErr := errorQueue.PendingCount(10)
	if countErr != nil {
		t.Fatalf("PendingCount: %v", countErr)
	}
	if pendingCount != 1 {
		t.Errorf("pending count = %d, want 1 (retry disabled)", pendingCount)
	}

	cancel()
	<-errCh
}
