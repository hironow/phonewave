package session_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hironow/phonewave/internal/session"
	"github.com/hironow/phonewave/internal/usecase/port"
)

func newTestDeliveryStore(t *testing.T) *session.SQLiteDeliveryStore {
	t.Helper()
	stateDir := t.TempDir()
	ds, err := session.NewSQLiteDeliveryStore(stateDir)
	if err != nil {
		t.Fatalf("NewSQLiteDeliveryStore: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

func TestSQLiteDeliveryStore_StageAndFlush(t *testing.T) {
	// given — a delivery store and a target directory
	ds := newTestDeliveryStore(t)
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "spec-001.md")
	data := []byte("test content")

	// when — stage and flush
	if err := ds.StageDelivery(context.Background(), "/outbox/spec-001.md", data, []string{target}); err != nil {
		t.Fatalf("StageDelivery: %v", err)
	}
	flushed, err := ds.FlushDeliveries(context.Background())
	if err != nil {
		t.Fatalf("FlushDeliveries: %v", err)
	}

	// then — file exists and flushed result returned
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed, got %d", len(flushed))
	}
	if flushed[0].DMailPath != "/outbox/spec-001.md" {
		t.Errorf("expected DMailPath '/outbox/spec-001.md', got %q", flushed[0].DMailPath)
	}
	if flushed[0].Target != target {
		t.Errorf("expected Target %q, got %q", target, flushed[0].Target)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile target: %v", err)
	}
	if string(got) != "test content" {
		t.Errorf("file content = %q, want %q", got, "test content")
	}
}

func TestSQLiteDeliveryStore_StageMultipleTargets(t *testing.T) {
	// given — same dmail going to 2 inboxes
	ds := newTestDeliveryStore(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	target1 := filepath.Join(dir1, "fb-001.md")
	target2 := filepath.Join(dir2, "fb-001.md")
	data := []byte("feedback")

	// when
	if err := ds.StageDelivery(context.Background(), "/outbox/fb-001.md", data, []string{target1, target2}); err != nil {
		t.Fatalf("StageDelivery: %v", err)
	}
	flushed, err := ds.FlushDeliveries(context.Background())
	if err != nil {
		t.Fatalf("FlushDeliveries: %v", err)
	}

	// then — both targets written
	if len(flushed) != 2 {
		t.Fatalf("expected 2 flushed, got %d", len(flushed))
	}
	for _, target := range []string{target1, target2} {
		if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
			t.Errorf("target %q should exist", target)
		}
	}
}

func TestSQLiteDeliveryStore_StageUpsert_LatestDataWins(t *testing.T) {
	// given — stage the same (dmailPath, target) twice with different data
	ds := newTestDeliveryStore(t)
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "spec-001.md")

	if err := ds.StageDelivery(context.Background(), "/outbox/spec-001.md", []byte("original"), []string{target}); err != nil {
		t.Fatalf("StageDelivery 1: %v", err)
	}
	if err := ds.StageDelivery(context.Background(), "/outbox/spec-001.md", []byte("modified"), []string{target}); err != nil {
		t.Fatalf("StageDelivery 2: %v", err)
	}

	// when
	flushed, err := ds.FlushDeliveries(context.Background())
	if err != nil {
		t.Fatalf("FlushDeliveries: %v", err)
	}

	// then — latest data wins (upsert semantics)
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed, got %d", len(flushed))
	}
	got, _ := os.ReadFile(target)
	if string(got) != "modified" {
		t.Errorf("expected modified data, got %q", got)
	}
}

func TestSQLiteDeliveryStore_RestageAfterFlush_EnablesRedelivery(t *testing.T) {
	// given — a delivery that has been staged and flushed
	ds := newTestDeliveryStore(t)
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "conflict.md")

	if err := ds.StageDelivery(context.Background(), "/outbox/conflict.md", []byte("first"), []string{target}); err != nil {
		t.Fatalf("StageDelivery 1: %v", err)
	}
	flushed, err := ds.FlushDeliveries(context.Background())
	if err != nil {
		t.Fatalf("FlushDeliveries 1: %v", err)
	}
	if len(flushed) != 1 {
		t.Fatalf("first flush: expected 1, got %d", len(flushed))
	}

	// when — re-stage with updated data (e.g. conflict D-Mail re-sent)
	if err := ds.StageDelivery(context.Background(), "/outbox/conflict.md", []byte("second"), []string{target}); err != nil {
		t.Fatalf("StageDelivery 2: %v", err)
	}

	// then — second flush should deliver the updated data
	flushed, err = ds.FlushDeliveries(context.Background())
	if err != nil {
		t.Fatalf("FlushDeliveries 2: %v", err)
	}
	if len(flushed) != 1 {
		t.Fatalf("second flush: expected 1 (re-delivery), got %d", len(flushed))
	}
	got, _ := os.ReadFile(target)
	if string(got) != "second" {
		t.Errorf("expected second data, got %q", got)
	}
}

func TestSQLiteDeliveryStore_FlushEmpty(t *testing.T) {
	// given — nothing staged
	ds := newTestDeliveryStore(t)

	// when
	flushed, err := ds.FlushDeliveries(context.Background())

	// then
	if err != nil {
		t.Fatalf("FlushDeliveries: %v", err)
	}
	if len(flushed) != 0 {
		t.Errorf("expected 0 flushed, got %d", len(flushed))
	}
}

func TestSQLiteDeliveryStore_FlushPartialFailure(t *testing.T) {
	// given — 2 targets, one dir missing
	ds := newTestDeliveryStore(t)
	goodDir := t.TempDir()
	goodTarget := filepath.Join(goodDir, "spec.md")
	badTarget := filepath.Join("/nonexistent-dir-xyz", "spec.md")
	data := []byte("content")

	if err := ds.StageDelivery(context.Background(), "/outbox/spec.md", data, []string{goodTarget, badTarget}); err != nil {
		t.Fatalf("StageDelivery: %v", err)
	}

	// when
	flushed, err := ds.FlushDeliveries(context.Background())

	// then — good target flushed, bad target not (retry_count incremented)
	if err != nil {
		t.Fatalf("FlushDeliveries: %v", err)
	}
	if len(flushed) != 1 {
		t.Fatalf("expected 1 flushed (partial), got %d", len(flushed))
	}
	if flushed[0].Target != goodTarget {
		t.Errorf("expected good target, got %q", flushed[0].Target)
	}

	// AllFlushedFor should be false
	allDone, _ := ds.AllFlushedFor("/outbox/spec.md")
	if allDone {
		t.Error("expected AllFlushedFor=false with partial flush")
	}
}

func TestSQLiteDeliveryStore_DeadLetter(t *testing.T) {
	// given — stage to a bad target, flush 3 times → dead letter
	ds := newTestDeliveryStore(t)
	badTarget := filepath.Join("/nonexistent-dir-xyz", "dead.md")
	data := []byte("content")

	if err := ds.StageDelivery(context.Background(), "/outbox/dead.md", data, []string{badTarget}); err != nil {
		t.Fatalf("StageDelivery: %v", err)
	}

	// when — flush 3 times (each increments retry_count)
	for range 3 {
		ds.FlushDeliveries(context.Background())
	}

	// then — 4th flush should skip it (dead letter)
	flushed, err := ds.FlushDeliveries(context.Background())
	if err != nil {
		t.Fatalf("FlushDeliveries: %v", err)
	}
	if len(flushed) != 0 {
		t.Errorf("expected 0 flushed (dead letter), got %d", len(flushed))
	}
}

func TestSQLiteDeliveryStore_RecoverUnflushed(t *testing.T) {
	// given — stage without flushing
	ds := newTestDeliveryStore(t)
	target := filepath.Join(t.TempDir(), "spec.md")
	data := []byte("unflushed data")

	if err := ds.StageDelivery(context.Background(), "/outbox/spec.md", data, []string{target}); err != nil {
		t.Fatalf("StageDelivery: %v", err)
	}

	// when
	unflushed, err := ds.RecoverUnflushed()

	// then
	if err != nil {
		t.Fatalf("RecoverUnflushed: %v", err)
	}
	if len(unflushed) != 1 {
		t.Fatalf("expected 1 unflushed, got %d", len(unflushed))
	}
	if unflushed[0].DMailPath != "/outbox/spec.md" {
		t.Errorf("expected DMailPath '/outbox/spec.md', got %q", unflushed[0].DMailPath)
	}
	if string(unflushed[0].Data) != "unflushed data" {
		t.Errorf("expected data 'unflushed data', got %q", unflushed[0].Data)
	}
}

func TestSQLiteDeliveryStore_AllFlushedFor(t *testing.T) {
	// given — stage 2 targets, flush both
	ds := newTestDeliveryStore(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	target1 := filepath.Join(dir1, "spec.md")
	target2 := filepath.Join(dir2, "spec.md")
	data := []byte("content")

	if err := ds.StageDelivery(context.Background(), "/outbox/spec.md", data, []string{target1, target2}); err != nil {
		t.Fatalf("StageDelivery: %v", err)
	}

	// when — flush all
	ds.FlushDeliveries(context.Background())

	// then
	allDone, err := ds.AllFlushedFor("/outbox/spec.md")
	if err != nil {
		t.Fatalf("AllFlushedFor: %v", err)
	}
	if !allDone {
		t.Error("expected AllFlushedFor=true after full flush")
	}
}

func TestSQLiteDeliveryStore_PruneFlushed(t *testing.T) {
	// given — stage, flush, then prune
	ds := newTestDeliveryStore(t)
	target := filepath.Join(t.TempDir(), "spec.md")
	data := []byte("content")

	ds.StageDelivery(context.Background(), "/outbox/spec.md", data, []string{target})
	ds.FlushDeliveries(context.Background())

	// when
	pruned, err := ds.PruneFlushed(context.Background())

	// then
	if err != nil {
		t.Fatalf("PruneFlushed: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	// RecoverUnflushed should return empty
	unflushed, _ := ds.RecoverUnflushed()
	if len(unflushed) != 0 {
		t.Errorf("expected 0 unflushed after prune, got %d", len(unflushed))
	}
}

func TestSQLiteDeliveryStore_ConcurrentStageFlush(t *testing.T) {
	// given — concurrent stage and flush operations
	ds := newTestDeliveryStore(t)
	baseDir := t.TempDir()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dir := filepath.Join(baseDir, fmt.Sprintf("inbox-%d", i))
			os.MkdirAll(dir, 0o755)
			target := filepath.Join(dir, "spec.md")
			dmailPath := fmt.Sprintf("/outbox/spec-%d.md", i)
			ds.StageDelivery(context.Background(), dmailPath, []byte("data"), []string{target})
			ds.FlushDeliveries(context.Background())
		}()
	}
	wg.Wait()

	// then — no panics, no errors
}

func TestSQLiteDeliveryStore_FilePermission(t *testing.T) {
	// given — create a delivery store
	stateDir := t.TempDir()
	ds, err := session.NewSQLiteDeliveryStore(stateDir)
	if err != nil {
		t.Fatalf("NewSQLiteDeliveryStore: %v", err)
	}
	defer ds.Close()

	// then — db file has 0o600 permissions
	dbPath := filepath.Join(stateDir, ".run", "delivery.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected 0600 permissions, got %o", perm)
	}
}

// Compile-time check
var _ port.DeliveryStore = (*session.SQLiteDeliveryStore)(nil)
