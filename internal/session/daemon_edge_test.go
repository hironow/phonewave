package session

import (
	"context"
	phonewave "github.com/hironow/phonewave"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
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

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		Verbose:    true,
	}, phonewave.NewLogger(io.Discard, false))
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

	errStore, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatalf("NewSQLiteErrorStore: %v", err)
	}
	defer errStore.Close()

	// Only route for "specification", not "mystery"
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		ErrorStore: errStore,
	}, phonewave.NewLogger(io.Discard, false))
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

	// Source file should be removed from outbox (moved to error queue)
	if _, err := os.Stat(filepath.Join(outbox, "mystery-001.md")); err == nil {
		t.Error("source should be removed from outbox on delivery failure (moved to error queue)")
	}

	// Error store should contain the failed D-Mail
	pending, listErr := errStore.ListPending(10)
	if listErr != nil {
		t.Fatalf("ListPending: %v", listErr)
	}
	if len(pending) != 1 {
		t.Errorf("error store entries = %d, want 1", len(pending))
	}
	if len(pending) > 0 && pending[0].OriginalName != "mystery-001.md" {
		t.Errorf("error entry OriginalName = %q, want %q", pending[0].OriginalName, "mystery-001.md")
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

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
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
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
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
dmail-schema-version: "1"
name: spec-valid
kind: specification
description: "Valid"
---
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-valid.md"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	results, errs := ScanAndDeliver(context.Background(), outbox, routes, nil, phonewave.NewLogger(io.Discard, false))

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
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	errStore, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatalf("NewSQLiteErrorStore: %v", err)
	}
	defer errStore.Close()

	// Valid D-Mail
	if err := os.WriteFile(filepath.Join(outbox, "spec-001.md"), []byte(`---
dmail-schema-version: "1"
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
dmail-schema-version: "1"
name: spec-003
kind: specification
description: "Also valid"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	results, errs := ScanAndDeliver(context.Background(), outbox, routes, errStore, phonewave.NewLogger(io.Discard, false))

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

	// Invalid file should be removed from outbox (moved to error store)
	if _, err := os.Stat(filepath.Join(outbox, "bad-002.md")); !os.IsNotExist(err) {
		t.Error("bad-002.md should be removed from outbox (moved to error store)")
	}

	// Verify it's in the error store
	pending, listErr := errStore.ListPending(10)
	if listErr != nil {
		t.Fatalf("ListPending: %v", listErr)
	}
	foundInErrorStore := false
	for _, e := range pending {
		if e.OriginalName == "bad-002.md" {
			foundInErrorStore = true
			break
		}
	}
	if !foundInErrorStore {
		t.Error("bad-002.md should be in the error store")
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
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when — try to deliver a file that doesn't exist
	_, err := Deliver(context.Background(), filepath.Join(outbox, "ghost.md"), routes)

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
dmail-schema-version: "1"
name: spec-dup
kind: specification
description: "New version"
---

# Updated specification
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-dup.md"), []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	result, err := Deliver(context.Background(), filepath.Join(outbox, "spec-dup.md"), routes)

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
dmail-schema-version: "1"
name: spec-noinbox
kind: specification
description: "No inbox target"
---
`
	dmailPath := filepath.Join(outbox, "spec-noinbox.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{nonExistentInbox}},
	}

	// when
	_, err := Deliver(context.Background(), dmailPath, routes)

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

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
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
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{"/tmp/nope"}},
	}

	// when — scan an empty outbox
	results, errs := ScanAndDeliver(context.Background(), outbox, routes, nil, phonewave.NewLogger(io.Discard, false))

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
	stateDir := filepath.Join(repoDir, phonewave.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a PID file with a PID that definitely doesn't exist
	// Use PID 999999999 which almost certainly isn't running
	pidPath := filepath.Join(stateDir, "watch.pid")
	if err := os.WriteFile(pidPath, []byte("999999999"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &phonewave.Config{}

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
	outbox2 := filepath.Join(repoDir, ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, ".expedition", "inbox")
	inbox2 := filepath.Join(repoDir, ".siren", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox1, outbox2, inbox1, inbox2, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox1, ToInboxes: []string{inbox1}},
		{Kind: "feedback", FromOutbox: outbox2, ToInboxes: []string{inbox2}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox1, outbox2},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
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
dmail-schema-version: "1"
name: spec-multi
kind: specification
description: "Multi outbox test"
---
`
	fbContent := `---
dmail-schema-version: "1"
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

// --- Edge Case: delivery to multiple inboxes, partial failure with rollback ---

func TestDeliver_PartialFailure_RollsBackDeliveredInboxes(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, ".siren", "inbox")
	// inbox2 does NOT exist — will cause partial failure
	inbox2 := filepath.Join(repoDir, ".expedition", "inbox-nonexistent")

	for _, dir := range []string{outbox, inbox1} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: fb-partial
kind: feedback
description: "Partial failure test"
---
`
	dmailPath := filepath.Join(outbox, "fb-partial.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}

	// when
	_, err := Deliver(context.Background(), dmailPath, routes)

	// then — should return error (partial failure)
	if err == nil {
		t.Fatal("expected error for partial failure")
	}

	// inbox1 should be cleaned up (rolled back) to prevent duplicates on retry
	if _, err := os.Stat(filepath.Join(inbox1, "fb-partial.md")); !os.IsNotExist(err) {
		t.Error("inbox1 should be rolled back on partial delivery failure to prevent duplicates on retry")
	}

	// Source should still exist (delivery failed)
	if _, err := os.Stat(dmailPath); os.IsNotExist(err) {
		t.Error("source should still exist after delivery failure")
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

// --- Edge Case: preserve D-Mail when error queue write fails ---

func TestDaemon_PreservesOutboxFileWhenNoErrorStore(t *testing.T) {
	// given — a daemon with nil ErrorStore. When delivery fails, the outbox
	// file must NOT be deleted because there is no error store to back it up.
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// No route for "specification" — delivery WILL fail
	routes := []phonewave.ResolvedRoute{}

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

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
		// ErrorStore intentionally nil
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()

	// when — handleEvent fires for a file that will fail delivery
	d.handleEvent(fsnotify.Event{
		Name: dmailPath,
		Op:   fsnotify.Create,
	})

	// then — outbox file must still exist (no error store to persist failure)
	if _, err := os.Stat(dmailPath); os.IsNotExist(err) {
		t.Error("outbox file was deleted even though no error store exists — D-Mail lost permanently")
	}
}

func TestScanAndDeliver_PreservesOutboxFileWhenNoErrorStore(t *testing.T) {
	// given — nil ErrorStore: when delivery fails, outbox file must stay
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	if err := os.MkdirAll(outbox, 0755); err != nil {
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

	// No routes — delivery will fail
	routes := []phonewave.ResolvedRoute{}

	// when — nil ErrorStore
	ScanAndDeliver(context.Background(), outbox, routes, nil, phonewave.NewLogger(io.Discard, false))

	// then — outbox file must still exist (no error store to persist failure)
	if _, err := os.Stat(dmailPath); os.IsNotExist(err) {
		t.Error("outbox file was deleted even though no error store exists — D-Mail lost permanently")
	}
}

// --- Edge Case: Rename event from atomic temp+rename ---

func TestDaemon_HandleRenameEvent(t *testing.T) {
	// given — a daemon with a valid route and a .md file in outbox.
	// Producers using temp+rename semantics emit Rename (not Create) on some platforms.
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

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     routes,
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	// Initialize delivery log (normally done in Run)
	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()

	// when — simulate a Rename event (as if producer used temp+rename)
	d.handleEvent(fsnotify.Event{
		Name: dmailPath,
		Op:   fsnotify.Rename,
	})

	// then — file should be delivered to inbox
	if _, err := os.Stat(filepath.Join(inbox, "spec-rename.md")); os.IsNotExist(err) {
		t.Error("D-Mail not delivered to inbox on Rename event")
	}
}

func TestDaemon_HandleRenameEvent_FileGone(t *testing.T) {
	// given — a Rename event for a file that was renamed AWAY (source side).
	// The daemon should silently ignore it (no error log spam).
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:     []phonewave.ResolvedRoute{},
		OutboxDirs: []string{outbox},
		StateDir:   stateDir,
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	dlog, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	d.dlog = dlog
	defer dlog.Close()

	// when — simulate a Rename event for a non-existent file (renamed away)
	// then — should not panic or log error (silent ignore)
	d.handleEvent(fsnotify.Event{
		Name: filepath.Join(outbox, "gone.md"),
		Op:   fsnotify.Rename,
	})
}

// --- Retry mechanism tests ---

func TestDaemon_RetrySucceeds(t *testing.T) {
	// given — an error store entry and a daemon with RetryInterval.
	// The error entry was created when no route existed, but now a route is available.
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	errStore, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer errStore.Close()

	// Place a D-Mail in the error store (simulating a prior failure)
	dmailData := []byte("---\ndmail-schema-version: \"1\"\nname: spec-retry\nkind: specification\ndescription: \"Retry test\"\n---\n\n# Retry Test\n")
	now := time.Now().UTC()
	entry := phonewave.RetryEntry{
		Name:         "retry-test-entry",
		SourceOutbox: outbox,
		Kind:         "specification",
		OriginalName: "spec-retry.md",
		Data:         dmailData,
		Attempts:     1,
		Error:        "no route for kind",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := errStore.RecordFailure(entry); err != nil {
		t.Fatal(err)
	}

	// Now provide the route that was previously missing
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:        routes,
		OutboxDirs:    []string{outbox},
		StateDir:      stateDir,
		ErrorStore:    errStore,
		Verbose:       true,
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    10,
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	// Wait for retry to fire and deliver
	deadline := time.After(3 * time.Second)
	delivered := false
	for !delivered {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for retry delivery")
		default:
			if _, err := os.Stat(filepath.Join(inbox, "spec-retry.md")); err == nil {
				delivered = true
			} else {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}

	// then — file should be in inbox
	if _, err := os.Stat(filepath.Join(inbox, "spec-retry.md")); os.IsNotExist(err) {
		t.Error("D-Mail not found in inbox after retry")
	}

	// Error store should be empty
	pending, _ := errStore.ListPending(10)
	if len(pending) != 0 {
		t.Errorf("error store still has %d entries, want 0", len(pending))
	}

	// Delivery log should contain RETRIED
	logData, _ := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if !strings.Contains(string(logData), "RETRIED") {
		t.Error("delivery log should contain RETRIED entry")
	}

	cancel()
	<-errCh
}

func TestDaemon_RetryExceedsMaxAttempts(t *testing.T) {
	// given — an error store entry with attempts already at max
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	errStore, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer errStore.Close()

	dmailData := []byte("---\ndmail-schema-version: \"1\"\nname: spec-maxed\nkind: specification\ndescription: \"Max retry\"\n---\n")
	now := time.Now().UTC()
	entry := phonewave.RetryEntry{
		Name:         "maxed-out-entry",
		SourceOutbox: outbox,
		Kind:         "specification",
		OriginalName: "spec-maxed.md",
		Data:         dmailData,
		Attempts:     10, // already at max
		Error:        "no route for kind",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := errStore.RecordFailure(entry); err != nil {
		t.Fatal(err)
	}

	// No routes — retry would fail anyway, but it shouldn't even try
	d, err := NewDaemon(DaemonOptions{
		Routes:        []phonewave.ResolvedRoute{},
		OutboxDirs:    []string{outbox},
		StateDir:      stateDir,
		ErrorStore:    errStore,
		RetryInterval: 100 * time.Millisecond,
		MaxRetries:    10,
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	// Wait for a couple retry ticks
	time.Sleep(350 * time.Millisecond)

	// then — error store entry should still be there (skipped, not retried)
	// ListPending with maxRetries=10 excludes entries with attempts >= 10
	pending, _ := errStore.ListPending(10)
	if len(pending) != 0 {
		t.Errorf("ListPending(10) = %d, want 0 (entry should be excluded)", len(pending))
	}
	// But the entry is still in the store (attempts not incremented)
	all, _ := errStore.ListPending(11) // include entries with attempts=10
	if len(all) != 1 {
		t.Errorf("ListPending(11) = %d, want 1 (entry should still exist)", len(all))
	}
	if len(all) > 0 && all[0].Attempts != 10 {
		t.Errorf("attempts = %d, want 10 (should not have been retried)", all[0].Attempts)
	}

	cancel()
	<-errCh
}

func TestDaemon_RetryDisabledWhenZeroInterval(t *testing.T) {
	// given — error store has an entry, but RetryInterval is 0 (disabled)
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	stateDir := filepath.Join(repoDir, ".phonewave")
	for _, dir := range []string{outbox, inbox, stateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	errStore, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatalf("NewSQLiteErrorStore: %v", err)
	}
	defer errStore.Close()

	now := time.Now().UTC()
	entry := phonewave.RetryEntry{
		Name:         now.Format("2006-01-02T150405.000000000") + "-specification-spec-nope.md",
		SourceOutbox: outbox,
		Kind:         "specification",
		OriginalName: "spec-nope.md",
		Data:         []byte("---\ndmail-schema-version: \"1\"\nname: spec-nope\nkind: specification\ndescription: \"No retry\"\n---\n"),
		Attempts:     1,
		Error:        "no route for kind",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := errStore.RecordFailure(entry); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	d, err := NewDaemon(DaemonOptions{
		Routes:        routes,
		OutboxDirs:    []string{outbox},
		StateDir:      stateDir,
		ErrorStore:    errStore,
		RetryInterval: 0, // disabled
	}, phonewave.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	// Wait — if retry were enabled with 0 interval, it would fire immediately
	time.Sleep(300 * time.Millisecond)

	// then — file should NOT be in inbox (retry disabled)
	if _, err := os.Stat(filepath.Join(inbox, "spec-nope.md")); err == nil {
		t.Error("D-Mail should NOT be in inbox (retry disabled)")
	}

	// Error store should still have the entry (untouched)
	pending, _ := errStore.ListPending(10)
	if len(pending) != 1 {
		t.Errorf("ListPending = %d, want 1 (retry disabled, entry untouched)", len(pending))
	}
	if len(pending) > 0 && pending[0].Attempts != 1 {
		t.Errorf("attempts = %d, want 1 (should not have been retried)", pending[0].Attempts)
	}

	cancel()
	<-errCh
}
