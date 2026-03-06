package eventsource
// white-box-reason: eventsource internals: tests unexported file rotation and lifecycle logic

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListExpiredEventFiles_FiltersOlderThanThreshold(t *testing.T) {
	// given — 2 old files, 1 recent file
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	writeEventFile(t, eventsDir, "2026-01-01.jsonl", oldTime)
	writeEventFile(t, eventsDir, "2026-01-15.jsonl", oldTime)
	writeEventFile(t, eventsDir, "2026-02-28.jsonl", time.Now())

	// when
	expired, err := ListExpiredEventFiles(dir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expired) != 2 {
		t.Errorf("expected 2 expired files, got %d: %v", len(expired), expired)
	}
}

func TestListExpiredEventFiles_EmptyDir(t *testing.T) {
	// given — events dir exists but is empty
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "events"), 0o755)

	// when
	expired, err := ListExpiredEventFiles(dir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected 0 expired files, got %d", len(expired))
	}
}

func TestListExpiredEventFiles_NonExistentDir(t *testing.T) {
	// given — events dir does not exist
	dir := t.TempDir()

	// when
	expired, err := ListExpiredEventFiles(dir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired != nil {
		t.Errorf("expected nil for non-existent dir, got %v", expired)
	}
}

func TestListExpiredEventFiles_NegativeDays(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	_, err := ListExpiredEventFiles(dir, -1)

	// then
	if err == nil {
		t.Error("expected error for negative days")
	}
}

func TestListExpiredEventFiles_IgnoresNonJsonlFiles(t *testing.T) {
	// given — a .txt file and a directory, both old
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	writeEventFile(t, eventsDir, "notes.txt", oldTime)
	os.MkdirAll(filepath.Join(eventsDir, "subdir"), 0o755)
	os.Chtimes(filepath.Join(eventsDir, "subdir"), oldTime, oldTime)

	// when
	expired, err := ListExpiredEventFiles(dir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected 0 expired .jsonl files, got %d: %v", len(expired), expired)
	}
}

func TestPruneEventFiles_DeletesSpecifiedFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, "events")
	os.MkdirAll(eventsDir, 0o755)

	writeEventFile(t, eventsDir, "2026-01-01.jsonl", time.Now())
	writeEventFile(t, eventsDir, "2026-01-02.jsonl", time.Now())

	// when
	deleted, err := PruneEventFiles(dir, []string{"2026-01-01.jsonl", "2026-01-02.jsonl"})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("expected 2 deleted, got %d", len(deleted))
	}
	for _, name := range deleted {
		if _, statErr := os.Stat(filepath.Join(eventsDir, name)); !os.IsNotExist(statErr) {
			t.Errorf("file %s should have been deleted", name)
		}
	}
}

func TestPruneEventFiles_IdempotentForMissingFiles(t *testing.T) {
	// given — file does not exist
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "events"), 0o755)

	// when — pruning non-existent file should not error
	deleted, err := PruneEventFiles(dir, []string{"nonexistent.jsonl"})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 1 {
		t.Errorf("expected 1 (idempotent), got %d", len(deleted))
	}
}

// writeEventFile creates a .jsonl file with a specific mtime for testing.
func writeEventFile(t *testing.T, dir, name string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	os.WriteFile(path, []byte(`{"id":"test"}`+"\n"), 0o644)
	os.Chtimes(path, mtime, mtime)
}
