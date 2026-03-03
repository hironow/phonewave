package session_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
)

func TestMigrateFileErrorQueue_MigratesExisting(t *testing.T) {
	// given: a stateDir with a legacy .err sidecar + data file
	stateDir := t.TempDir()
	meta := domain.ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "specification",
		OriginalName: "spec-001.md",
		Attempts:     2,
		Error:        "no route",
		Timestamp:    time.Now().UTC(),
	}

	// Create legacy files using the root SaveToErrorQueue
	if err := session.SaveToErrorQueue(stateDir, meta, []byte("dmail data")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Verify legacy files exist
	errorsDir := filepath.Join(stateDir, "errors")
	entries, _ := os.ReadDir(errorsDir)
	if len(entries) < 2 {
		t.Fatalf("setup: expected at least 2 files in errors dir, got %d", len(entries))
	}

	store := testErrorQueueStore(t)
	logger := platform.NewLogger(io.Discard, false)

	// when
	migrated, err := session.MigrateFileErrorQueue(stateDir, store, logger)

	// then
	if err != nil {
		t.Fatalf("MigrateFileErrorQueue: %v", err)
	}
	if migrated != 1 {
		t.Errorf("migrated: got %d, want 1", migrated)
	}

	// Data should be in the SQLite store
	count, err := store.PendingCount(10)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}
	if count != 1 {
		t.Errorf("pending count: got %d, want 1", count)
	}

	// Legacy files should be removed
	remaining, _ := os.ReadDir(errorsDir)
	if len(remaining) != 0 {
		t.Errorf("remaining files: got %d, want 0", len(remaining))
	}
}

func TestMigrateFileErrorQueue_IdempotentRerun(t *testing.T) {
	// given: migrate once, then run again
	stateDir := t.TempDir()
	meta := domain.ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "report-001.md",
		Attempts:     1,
		Error:        "timeout",
		Timestamp:    time.Now().UTC(),
	}
	session.SaveToErrorQueue(stateDir, meta, []byte("report data"))

	store := testErrorQueueStore(t)
	logger := platform.NewLogger(io.Discard, false)

	// First migration
	session.MigrateFileErrorQueue(stateDir, store, logger)

	// when: second migration (files already removed)
	migrated, err := session.MigrateFileErrorQueue(stateDir, store, logger)

	// then
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if migrated != 0 {
		t.Errorf("second migration: got %d, want 0", migrated)
	}

	// Still only 1 entry in store (idempotent)
	count, _ := store.PendingCount(10)
	if count != 1 {
		t.Errorf("pending count after idempotent rerun: got %d, want 1", count)
	}
}

func TestMigrateFileErrorQueue_EmptyDir(t *testing.T) {
	// given: stateDir with no errors/ directory
	stateDir := t.TempDir()
	store := testErrorQueueStore(t)
	logger := platform.NewLogger(io.Discard, false)

	// when
	migrated, err := session.MigrateFileErrorQueue(stateDir, store, logger)

	// then
	if err != nil {
		t.Fatalf("MigrateFileErrorQueue: %v", err)
	}
	if migrated != 0 {
		t.Errorf("migrated: got %d, want 0", migrated)
	}
}
