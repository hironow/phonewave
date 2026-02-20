package phonewave

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeliveryLog_Append(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	defer log.Close()

	// when
	log.Delivered("feedback", "/repo/.gate/outbox/feedback-001.md", "/repo/.siren/inbox/feedback-001.md")
	log.Delivered("feedback", "/repo/.gate/outbox/feedback-001.md", "/repo/.expedition/inbox/feedback-001.md")
	log.Removed("/repo/.gate/outbox/feedback-001.md")

	// then — read the log file
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("log lines = %d, want 3", len(lines))
	}

	if !strings.Contains(lines[0], "DELIVERED") {
		t.Errorf("line 0 should contain DELIVERED: %s", lines[0])
	}
	if !strings.Contains(lines[0], "kind=feedback") {
		t.Errorf("line 0 should contain kind=feedback: %s", lines[0])
	}
	if !strings.Contains(lines[2], "REMOVED") {
		t.Errorf("line 2 should contain REMOVED: %s", lines[2])
	}
}

func TestDeliveryLog_Failed(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	defer log.Close()

	// when
	log.Failed("specification", "/repo/.siren/outbox/spec-001.md", "target inbox not found")

	// then
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "FAILED") {
		t.Error("log should contain FAILED")
	}
	if !strings.Contains(content, "kind=specification") {
		t.Error("log should contain kind=specification")
	}
}

func TestSaveToErrorQueue_WritesFileWithKindInName(t *testing.T) {
	// given
	stateDir := t.TempDir()
	meta := ErrorMetadata{
		SourceOutbox: "/repo/.siren/outbox",
		Kind:         "specification",
		OriginalName: "spec-fail.md",
		Attempts:     1,
		Error:        "no route for kind",
		Timestamp:    time.Now().UTC(),
	}
	data := []byte("---\nname: spec-fail\nkind: specification\n---\n# Failed spec\n")

	// when
	err := SaveToErrorQueue(stateDir, meta, data)

	// then
	if err != nil {
		t.Fatalf("SaveToErrorQueue: %v", err)
	}

	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// Filter out .err sidecar files
	var mdFiles []os.DirEntry
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".err") {
			continue
		}
		mdFiles = append(mdFiles, e)
	}
	if len(mdFiles) != 1 {
		t.Fatalf("error queue .md files = %d, want 1", len(mdFiles))
	}

	name := mdFiles[0].Name()
	if !strings.Contains(name, "specification") {
		t.Errorf("filename %q should contain kind 'specification'", name)
	}
	if !strings.Contains(name, "spec-fail.md") {
		t.Errorf("filename %q should contain original name 'spec-fail.md'", name)
	}

	// Verify content matches
	content, err := os.ReadFile(filepath.Join(errorsDir, name))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", content, data)
	}
}

func TestSaveToErrorQueue_WritesSidecarFile(t *testing.T) {
	// given
	stateDir := t.TempDir()
	meta := ErrorMetadata{
		SourceOutbox: "/repo/.siren/outbox",
		Kind:         "specification",
		OriginalName: "spec-fail.md",
		Attempts:     1,
		Error:        "no route for kind",
		Timestamp:    time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
	}
	data := []byte("---\nname: spec-fail\nkind: specification\n---\n")

	// when
	err := SaveToErrorQueue(stateDir, meta, data)

	// then
	if err != nil {
		t.Fatalf("SaveToErrorQueue: %v", err)
	}

	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// Find .err sidecar
	var sidecarName string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".err") {
			sidecarName = e.Name()
			break
		}
	}
	if sidecarName == "" {
		t.Fatal("no .err sidecar file found")
	}

	sidecarContent, err := os.ReadFile(filepath.Join(errorsDir, sidecarName))
	if err != nil {
		t.Fatalf("ReadFile sidecar: %v", err)
	}

	content := string(sidecarContent)
	if !strings.Contains(content, "source_outbox: /repo/.siren/outbox") {
		t.Errorf("sidecar should contain source_outbox, got:\n%s", content)
	}
	if !strings.Contains(content, "kind: specification") {
		t.Errorf("sidecar should contain kind, got:\n%s", content)
	}
	if !strings.Contains(content, "attempts: 1") {
		t.Errorf("sidecar should contain attempts, got:\n%s", content)
	}
	if !strings.Contains(content, "error: no route for kind") {
		t.Errorf("sidecar should contain error, got:\n%s", content)
	}
}

func TestLoadErrorMetadata_RoundTrip(t *testing.T) {
	// given — save a D-Mail to error queue, then load its sidecar
	stateDir := t.TempDir()
	original := ErrorMetadata{
		SourceOutbox: "/repo/.gate/outbox",
		Kind:         "feedback",
		OriginalName: "feedback-001.md",
		Attempts:     3,
		Error:        "permission denied",
		Timestamp:    time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
	}
	data := []byte("---\nname: feedback-001\nkind: feedback\n---\n")

	if err := SaveToErrorQueue(stateDir, original, data); err != nil {
		t.Fatalf("SaveToErrorQueue: %v", err)
	}

	// Find the .err sidecar
	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatal(err)
	}
	var sidecarPath string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".err") {
			sidecarPath = filepath.Join(errorsDir, e.Name())
			break
		}
	}
	if sidecarPath == "" {
		t.Fatal("no .err sidecar found")
	}

	// when
	loaded, err := LoadErrorMetadata(sidecarPath)

	// then
	if err != nil {
		t.Fatalf("LoadErrorMetadata: %v", err)
	}
	if loaded.SourceOutbox != original.SourceOutbox {
		t.Errorf("SourceOutbox = %q, want %q", loaded.SourceOutbox, original.SourceOutbox)
	}
	if loaded.Kind != original.Kind {
		t.Errorf("Kind = %q, want %q", loaded.Kind, original.Kind)
	}
	if loaded.OriginalName != original.OriginalName {
		t.Errorf("OriginalName = %q, want %q", loaded.OriginalName, original.OriginalName)
	}
	if loaded.Attempts != original.Attempts {
		t.Errorf("Attempts = %d, want %d", loaded.Attempts, original.Attempts)
	}
	if loaded.Error != original.Error {
		t.Errorf("Error = %q, want %q", loaded.Error, original.Error)
	}
}

func TestDeliveryLog_Retried(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	defer log.Close()

	// when
	log.Retried("specification", "/repo/.siren/outbox/spec-001.md", "/repo/.expedition/inbox/spec-001.md")

	// then
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "RETRIED") {
		t.Error("log should contain RETRIED")
	}
	if !strings.Contains(content, "kind=specification") {
		t.Error("log should contain kind=specification")
	}
}

func TestUpdateErrorMetadata_IncrementsAttempts(t *testing.T) {
	// given — an error queue entry with attempts=1
	stateDir := t.TempDir()
	meta := ErrorMetadata{
		SourceOutbox: "/repo/.siren/outbox",
		Kind:         "specification",
		OriginalName: "spec-fail.md",
		Attempts:     1,
		Error:        "no route for kind",
		Timestamp:    time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
	}
	data := []byte("---\nname: spec-fail\nkind: specification\n---\n")
	if err := SaveToErrorQueue(stateDir, meta, data); err != nil {
		t.Fatal(err)
	}

	// Find sidecar
	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatal(err)
	}
	var sidecarPath string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".err") {
			sidecarPath = filepath.Join(errorsDir, e.Name())
			break
		}
	}
	if sidecarPath == "" {
		t.Fatal("no .err sidecar found")
	}

	// when
	if err := UpdateErrorMetadata(sidecarPath, "retry failed: still no route"); err != nil {
		t.Fatalf("UpdateErrorMetadata: %v", err)
	}

	// then
	loaded, err := LoadErrorMetadata(sidecarPath)
	if err != nil {
		t.Fatalf("LoadErrorMetadata: %v", err)
	}
	if loaded.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", loaded.Attempts)
	}
	if loaded.Error != "retry failed: still no route" {
		t.Errorf("Error = %q, want %q", loaded.Error, "retry failed: still no route")
	}
}

func TestRemoveErrorEntry_RemovesBothFiles(t *testing.T) {
	// given — an error queue entry with .md and .err files
	stateDir := t.TempDir()
	meta := ErrorMetadata{
		SourceOutbox: "/repo/.siren/outbox",
		Kind:         "specification",
		OriginalName: "spec-fail.md",
		Attempts:     1,
		Error:        "no route for kind",
		Timestamp:    time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
	}
	data := []byte("---\nname: spec-fail\nkind: specification\n---\n")
	if err := SaveToErrorQueue(stateDir, meta, data); err != nil {
		t.Fatal(err)
	}

	// Find the .md file (not the sidecar)
	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatal(err)
	}
	var dmailPath string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".err") {
			dmailPath = filepath.Join(errorsDir, e.Name())
			break
		}
	}
	if dmailPath == "" {
		t.Fatal("no D-Mail file found in error queue")
	}

	// when
	if err := RemoveErrorEntry(dmailPath); err != nil {
		t.Fatalf("RemoveErrorEntry: %v", err)
	}

	// then — both files should be gone
	if _, err := os.Stat(dmailPath); !os.IsNotExist(err) {
		t.Error("D-Mail file should be removed")
	}
	if _, err := os.Stat(dmailPath + ".err"); !os.IsNotExist(err) {
		t.Error(".err sidecar should be removed")
	}
}

func TestSaveToErrorQueue_SanitizesPathTraversal(t *testing.T) {
	// given — Kind and OriginalName contain path traversal sequences
	stateDir := t.TempDir()
	meta := ErrorMetadata{
		SourceOutbox: "/repo/.siren/outbox",
		Kind:         "../../etc",
		OriginalName: "../../../passwd",
		Attempts:     1,
		Error:        "path traversal attempt",
		Timestamp:    time.Now().UTC(),
	}

	// when
	err := SaveToErrorQueue(stateDir, meta, []byte("malicious"))

	// then — file must be written inside errors/ directory, not escaped
	if err != nil {
		t.Fatalf("SaveToErrorQueue: %v", err)
	}

	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	// Should have exactly 1 .md + 1 .err = 2 entries, all inside errors/
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 (data + sidecar)", len(entries))
	}

	for _, e := range entries {
		// Verify no path separator in filenames
		if strings.Contains(e.Name(), "/") || strings.Contains(e.Name(), "..") {
			t.Errorf("filename %q contains path traversal characters", e.Name())
		}
	}

	// Verify nothing was written outside errors/
	parentEntries, _ := os.ReadDir(stateDir)
	for _, e := range parentEntries {
		if e.Name() != "errors" {
			t.Errorf("unexpected file outside errors/: %s", e.Name())
		}
	}
}

func TestSaveToErrorQueue_CreatesErrorsDir(t *testing.T) {
	// given — stateDir exists but errors/ does not
	stateDir := t.TempDir()
	meta := ErrorMetadata{
		SourceOutbox: "/repo/.siren/outbox",
		Kind:         "specification",
		OriginalName: "spec-001.md",
		Attempts:     1,
		Error:        "test error",
		Timestamp:    time.Now().UTC(),
	}

	// when
	err := SaveToErrorQueue(stateDir, meta, []byte("test"))

	// then
	if err != nil {
		t.Fatalf("SaveToErrorQueue: %v", err)
	}
	errorsDir := filepath.Join(stateDir, "errors")
	if _, err := os.Stat(errorsDir); os.IsNotExist(err) {
		t.Error("errors/ directory should have been created")
	}
}
