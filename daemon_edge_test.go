package phonewave

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Edge Case: malformed D-Mail in outbox ---

func TestDaemon_MalformedDMail(t *testing.T) {
	// given — a file in outbox with invalid frontmatter
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

	// Wait a bit for daemon to attempt delivery
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
name: spec-after-bad
kind: specification
description: "Valid after malformed"
---

# Valid
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-after-bad.md"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for delivery
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("daemon should still be alive and delivering after malformed input")
		default:
			if _, err := os.Stat(filepath.Join(inbox, "spec-after-bad.md")); err == nil {
				goto delivered
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
delivered:

	cancel()
	if err := <-errCh; err != nil {
		t.Errorf("daemon error: %v", err)
	}
}

// --- Edge Case: unknown kind (no matching route) ---

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

	// Only route for "specification", not "mystery"
	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	})
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// when — write a D-Mail with unknown kind
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

	// then — inbox should be empty (no route for "mystery")
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("inbox should be empty for unknown kind, got %d files", len(entries))
	}

	// Source file should still exist (delivery failed, so source not removed)
	if _, err := os.Stat(filepath.Join(outbox, "mystery-001.md")); os.IsNotExist(err) {
		t.Error("source should NOT be removed when delivery fails")
	}

	cancel()
	<-errCh
}

// --- Edge Case: non-.md files are ignored ---

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

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	})
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// when — write various non-.md files
	for _, name := range []string{"notes.txt", "data.json", ".DS_Store", "README"} {
		if err := os.WriteFile(filepath.Join(outbox, name), []byte("not a dmail"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(300 * time.Millisecond)

	// then — inbox should be empty
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

// --- Edge Case: leftover temp files in outbox ---

func TestScanAndDeliver_IgnoresTempFiles(t *testing.T) {
	// given — outbox with a temp file and a valid D-Mail
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Write a temp file (should be ignored)
	if err := os.WriteFile(filepath.Join(outbox, ".phonewave-tmp-12345"), []byte("temp"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a valid D-Mail
	validContent := `---
name: spec-valid
kind: specification
description: "Valid"
---
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-valid.md"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	results, errs := ScanAndDeliver(outbox, routes)

	// then
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 (temp file should be skipped)", len(results))
	}
	if results[0].Kind != "specification" {
		t.Errorf("kind = %q, want specification", results[0].Kind)
	}

	// Temp file should still exist (not touched)
	if _, err := os.Stat(filepath.Join(outbox, ".phonewave-tmp-12345")); os.IsNotExist(err) {
		t.Error("temp file should not be removed by ScanAndDeliver")
	}
}

// --- Edge Case: mixed valid and invalid files in startup scan ---

func TestScanAndDeliver_MixedValidInvalid(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Valid D-Mail
	if err := os.WriteFile(filepath.Join(outbox, "spec-001.md"), []byte(`---
name: spec-001
kind: specification
description: "Valid"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Invalid D-Mail (no frontmatter)
	if err := os.WriteFile(filepath.Join(outbox, "bad-002.md"), []byte("no frontmatter here"), 0644); err != nil {
		t.Fatal(err)
	}

	// Another valid D-Mail
	if err := os.WriteFile(filepath.Join(outbox, "spec-003.md"), []byte(`---
name: spec-003
kind: specification
description: "Also valid"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	results, errs := ScanAndDeliver(outbox, routes)

	// then — should deliver 2, fail 1, and NOT stop on first error
	if len(results) != 2 {
		t.Errorf("results = %d, want 2 (valid D-Mails delivered)", len(results))
	}
	if len(errs) != 1 {
		t.Errorf("errors = %d, want 1 (one invalid file)", len(errs))
	}

	// Valid files should be in inbox
	if _, err := os.Stat(filepath.Join(inbox, "spec-001.md")); os.IsNotExist(err) {
		t.Error("spec-001.md should be in inbox")
	}
	if _, err := os.Stat(filepath.Join(inbox, "spec-003.md")); os.IsNotExist(err) {
		t.Error("spec-003.md should be in inbox")
	}

	// Invalid file should still be in outbox (not delivered, not removed)
	if _, err := os.Stat(filepath.Join(outbox, "bad-002.md")); os.IsNotExist(err) {
		t.Error("bad-002.md should still be in outbox (delivery failed)")
	}
}

// --- Edge Case: file vanished between detection and delivery ---

func TestDeliver_FileVanished(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Reference a non-existent file
	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when — try to deliver a file that doesn't exist
	_, err := Deliver(filepath.Join(outbox, "ghost.md"), routes)

	// then — should return error, not panic
	if err == nil {
		t.Fatal("expected error for vanished file")
	}
}

// --- Edge Case: overwrite existing file in inbox ---

func TestDeliver_OverwriteExistingInInbox(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Pre-existing file in inbox with different content
	oldContent := []byte("old version")
	if err := os.WriteFile(filepath.Join(inbox, "spec-dup.md"), oldContent, 0644); err != nil {
		t.Fatal(err)
	}

	// New D-Mail with same name
	newContent := `---
name: spec-dup
kind: specification
description: "New version"
---

# Updated specification
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-dup.md"), []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	result, err := Deliver(filepath.Join(outbox, "spec-dup.md"), routes)

	// then — should succeed (atomic rename overwrites)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if result.Kind != "specification" {
		t.Errorf("kind = %q, want specification", result.Kind)
	}

	// Inbox should have the NEW content
	data, err := os.ReadFile(filepath.Join(inbox, "spec-dup.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != newContent {
		t.Error("inbox file should contain new content (overwritten)")
	}
}

// --- Edge Case: missing target inbox directory ---

func TestDeliver_MissingInboxDir(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}

	// DO NOT create the inbox directory
	nonExistentInbox := filepath.Join(repoDir, ".expedition", "inbox")

	dmailContent := `---
name: spec-noinbox
kind: specification
description: "No inbox target"
---
`
	dmailPath := filepath.Join(outbox, "spec-noinbox.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{nonExistentInbox}},
	}

	// when
	_, err := Deliver(dmailPath, routes)

	// then — should return error (can't create temp file in nonexistent dir)
	if err == nil {
		t.Fatal("expected error when inbox directory doesn't exist")
	}

	// Source should NOT be removed (delivery failed)
	if _, err := os.Stat(dmailPath); os.IsNotExist(err) {
		t.Error("source should still exist when delivery fails")
	}
}

// --- Edge Case: burst delivery (multiple files in rapid succession) ---

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

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	})
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// when — write 5 D-Mails in rapid succession
	for i := range 5 {
		content := []byte(`---
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

	// Wait for all deliveries
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

	// then — all 5 should be in inbox
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Errorf("inbox has %d files, want 5", len(entries))
	}

	// All 5 should be removed from outbox (only temp file remnants or nothing)
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

// --- Edge Case: empty outbox on startup scan ---

func TestScanAndDeliver_EmptyOutbox(t *testing.T) {
	outbox := t.TempDir()
	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{"/tmp/nope"}},
	}

	// when — scan an empty outbox
	results, errs := ScanAndDeliver(outbox, routes)

	// then — no results, no errors
	if len(results) != 0 {
		t.Errorf("results = %d, want 0", len(results))
	}
	if len(errs) != 0 {
		t.Errorf("errors = %v, want none", errs)
	}
}

// --- Edge Case: stale PID file ---

func TestDoctor_StalePIDFile(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a PID file with a PID that definitely doesn't exist
	// Use PID 999999999 which almost certainly isn't running
	pidPath := filepath.Join(stateDir, "watch.pid")
	if err := os.WriteFile(pidPath, []byte("999999999"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}

	// when
	report := Doctor(cfg, stateDir)

	// then — daemon should NOT be reported as running (stale PID)
	if report.DaemonStatus.Running {
		t.Error("daemon should not be reported as running with stale PID")
	}
}

// --- Edge Case: multiple outboxes with concurrent activity ---

func TestDaemon_MultipleOutboxes(t *testing.T) {
	repoDir := t.TempDir()
	outbox1 := filepath.Join(repoDir, ".siren", "outbox")
	outbox2 := filepath.Join(repoDir, ".divergence", "outbox")
	inbox1 := filepath.Join(repoDir, ".expedition", "inbox")
	inbox2 := filepath.Join(repoDir, ".siren", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox1, outbox2, inbox1, inbox2, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox1, ToInboxes: []string{inbox1}},
		{Kind: "feedback", FromOutbox: outbox2, ToInboxes: []string{inbox2}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox1, outbox2},
		StateDir:   stateDir,
	})
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// when — write to BOTH outboxes simultaneously
	specContent := `---
name: spec-multi
kind: specification
description: "Multi outbox test"
---
`
	fbContent := `---
name: fb-multi
kind: feedback
description: "Multi outbox test"
---
`
	if err := os.WriteFile(filepath.Join(outbox1, "spec-multi.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outbox2, "fb-multi.md"), []byte(fbContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for both deliveries
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

	// then — both files should be in their respective inboxes
	if _, err := os.Stat(filepath.Join(inbox1, "spec-multi.md")); os.IsNotExist(err) {
		t.Error("spec-multi.md not found in inbox1")
	}
	if _, err := os.Stat(filepath.Join(inbox2, "fb-multi.md")); os.IsNotExist(err) {
		t.Error("fb-multi.md not found in inbox2")
	}

	cancel()
	<-errCh
}

// --- Edge Case: delivery to multiple inboxes, partial failure ---

func TestDeliver_PartialFailure_MultipleTargets(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".divergence", "outbox")
	inbox1 := filepath.Join(repoDir, ".siren", "inbox")
	// inbox2 does NOT exist — will cause partial failure
	inbox2 := filepath.Join(repoDir, ".expedition", "inbox-nonexistent")

	for _, dir := range []string{outbox, inbox1} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
name: fb-partial
kind: feedback
description: "Partial failure test"
---
`
	dmailPath := filepath.Join(outbox, "fb-partial.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []ResolvedRoute{
		{Kind: "feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}

	// when
	result, err := Deliver(dmailPath, routes)

	// then — should return error (partial failure)
	if err == nil {
		t.Fatal("expected error for partial failure")
	}

	// First target should have received the file
	if result == nil {
		t.Fatal("result should not be nil even on partial failure")
	}
	if len(result.DeliveredTo) != 1 {
		t.Errorf("delivered to %d targets, want 1 (partial success)", len(result.DeliveredTo))
	}

	// Source should still exist (not all targets succeeded)
	if _, err := os.Stat(dmailPath); os.IsNotExist(err) {
		t.Error("source should still exist after partial delivery failure")
	}
}

// --- Edge Case: delivery log survives daemon restart ---

func TestDeliveryLog_AppendAcrossRestarts(t *testing.T) {
	stateDir := t.TempDir()

	// First session
	log1, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	log1.Delivered("specification", "/outbox/spec-001.md", "/inbox/spec-001.md")
	log1.Close()

	// Second session (simulates daemon restart)
	log2, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	log2.Delivered("feedback", "/outbox/fb-001.md", "/inbox/fb-001.md")
	log2.Close()

	// then — log should contain entries from both sessions
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	lines := 0
	for _, c := range content {
		if c == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("log lines = %d, want 2 (across restarts)", lines)
	}
}
