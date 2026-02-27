package session_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/session"
)

// stubDeliverFn creates a deliver function that records calls and returns nil.
func stubDeliverFn(calls *[]deliverCall) session.DeliverFunc {
	return func(ctx context.Context, dmailPath string, data []byte) (*phonewave.DeliveryResult, error) {
		*calls = append(*calls, deliverCall{path: dmailPath, data: data})
		return &phonewave.DeliveryResult{
			SourcePath:  dmailPath,
			Kind:        "report",
			DeliveredTo: []string{"/target/inbox/" + filepath.Base(dmailPath)},
		}, nil
	}
}

type deliverCall struct {
	path string
	data []byte
}

func TestSQLiteOutboxStore_StageAndFlush(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), ".run", "outbox.db")
	var calls []deliverCall
	store, err := session.NewSQLiteOutboxStore(dbPath, stubDeliverFn(&calls))
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// when
	if err := store.Stage("rp-001.md", "report", "/repo/.siren/outbox", []byte("dmail content")); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// then
	if n != 1 {
		t.Errorf("Flush returned %d, want 1", n)
	}
	if len(calls) != 1 {
		t.Fatalf("deliverFn called %d times, want 1", len(calls))
	}
	wantPath := filepath.Join("/repo/.siren/outbox", "rp-001.md")
	if calls[0].path != wantPath {
		t.Errorf("deliverFn path = %q, want %q", calls[0].path, wantPath)
	}
	if string(calls[0].data) != "dmail content" {
		t.Errorf("deliverFn data = %q, want %q", calls[0].data, "dmail content")
	}
}

func TestSQLiteOutboxStore_Stage_Idempotent(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), ".run", "outbox.db")
	var calls []deliverCall
	store, err := session.NewSQLiteOutboxStore(dbPath, stubDeliverFn(&calls))
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// when — stage same (source_dir, name) twice
	if err := store.Stage("rp-001.md", "report", "/repo/.siren/outbox", []byte("first")); err != nil {
		t.Fatalf("Stage 1: %v", err)
	}
	if err := store.Stage("rp-001.md", "report", "/repo/.siren/outbox", []byte("second")); err != nil {
		t.Fatalf("Stage 2: %v", err)
	}
	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// then — deliverFn called once with first data (INSERT OR IGNORE)
	if n != 1 {
		t.Errorf("Flush returned %d, want 1", n)
	}
	if len(calls) != 1 {
		t.Fatalf("deliverFn called %d times, want 1", len(calls))
	}
	if string(calls[0].data) != "first" {
		t.Errorf("deliverFn data = %q, want %q", calls[0].data, "first")
	}
}

func TestSQLiteOutboxStore_Flush_Empty(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), ".run", "outbox.db")
	var calls []deliverCall
	store, err := session.NewSQLiteOutboxStore(dbPath, stubDeliverFn(&calls))
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// when
	n, err := store.Flush()

	// then
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if n != 0 {
		t.Errorf("Flush returned %d, want 0", n)
	}
	if len(calls) != 0 {
		t.Errorf("deliverFn called %d times, want 0", len(calls))
	}
}

func TestSQLiteOutboxStore_Flush_PartialFailure(t *testing.T) {
	// given — deliverFn fails on 2nd item
	dbPath := filepath.Join(t.TempDir(), ".run", "outbox.db")
	callCount := 0
	failOn := 2 // 1-based
	fn := func(ctx context.Context, dmailPath string, data []byte) (*phonewave.DeliveryResult, error) {
		callCount++
		if callCount == failOn {
			return nil, fmt.Errorf("simulated failure")
		}
		return &phonewave.DeliveryResult{
			SourcePath:  dmailPath,
			Kind:        "report",
			DeliveredTo: []string{"/inbox/" + filepath.Base(dmailPath)},
		}, nil
	}
	store, err := session.NewSQLiteOutboxStore(dbPath, fn)
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// Stage 3 items (same sourceDir, different names for deterministic order)
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("rp-%03d.md", i)
		if err := store.Stage(name, "report", "/repo/.siren/outbox", []byte(fmt.Sprintf("content-%d", i))); err != nil {
			t.Fatalf("Stage %d: %v", i, err)
		}
	}

	// when — first Flush: item 1 succeeds, item 2 fails (skipped), item 3 succeeds
	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush 1: %v", err)
	}

	// then — 2 delivered (1 and 3), item 2 unflushed
	if n != 2 {
		t.Errorf("Flush 1 returned %d, want 2", n)
	}
	pending, err := store.FlushPending()
	if err != nil {
		t.Fatalf("FlushPending: %v", err)
	}
	if pending != 1 {
		t.Errorf("FlushPending = %d, want 1", pending)
	}

	// when — second Flush: remaining item 2 succeeds (callCount is now 4, not failOn)
	n2, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush 2: %v", err)
	}

	// then
	if n2 != 1 {
		t.Errorf("Flush 2 returned %d, want 1", n2)
	}
	pending2, err := store.FlushPending()
	if err != nil {
		t.Fatalf("FlushPending 2: %v", err)
	}
	if pending2 != 0 {
		t.Errorf("FlushPending after retry = %d, want 0", pending2)
	}
}

func TestSQLiteOutboxStore_FlushPending(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), ".run", "outbox.db")
	callCount := 0
	fn := func(ctx context.Context, dmailPath string, data []byte) (*phonewave.DeliveryResult, error) {
		callCount++
		if callCount == 1 {
			return &phonewave.DeliveryResult{
				SourcePath:  dmailPath,
				Kind:        "report",
				DeliveredTo: []string{"/inbox/" + filepath.Base(dmailPath)},
			}, nil
		}
		return nil, fmt.Errorf("fail")
	}
	store, err := session.NewSQLiteOutboxStore(dbPath, fn)
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// Stage 3 items
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("rp-%03d.md", i)
		if err := store.Stage(name, "report", "/repo/.siren/outbox", []byte("data")); err != nil {
			t.Fatalf("Stage: %v", err)
		}
	}

	// when — Flush delivers only first item
	n, err := store.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// then
	if n != 1 {
		t.Errorf("Flush returned %d, want 1", n)
	}
	pending, err := store.FlushPending()
	if err != nil {
		t.Fatalf("FlushPending: %v", err)
	}
	if pending != 2 {
		t.Errorf("FlushPending = %d, want 2", pending)
	}
}

func TestSQLiteOutboxStore_PragmaSynchronousNormal(t *testing.T) {
	// given
	dbPath := filepath.Join(t.TempDir(), ".run", "outbox.db")
	store, err := session.NewSQLiteOutboxStore(dbPath, stubDeliverFn(&[]deliverCall{}))
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// when — check PRAGMA via store's own connection (synchronous is per-connection)
	var syncMode string
	if err := store.DB().QueryRow("PRAGMA synchronous").Scan(&syncMode); err != nil {
		t.Fatalf("PRAGMA synchronous: %v", err)
	}

	// then — NORMAL = 1
	if syncMode != "1" {
		t.Errorf("PRAGMA synchronous = %q, want '1' (NORMAL)", syncMode)
	}
}

func TestSQLiteOutboxStore_CreatesDBDirectory(t *testing.T) {
	// given — nested directory that doesn't exist
	base := t.TempDir()
	dbPath := filepath.Join(base, "deep", "nested", ".run", "outbox.db")

	// when
	store, err := session.NewSQLiteOutboxStore(dbPath, stubDeliverFn(&[]deliverCall{}))
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore: %v", err)
	}
	defer store.Close()

	// then — DB file exists
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("DB file not created: %v", err)
	}
}

func TestSQLiteOutboxStore_ConcurrentStageAndFlush(t *testing.T) {
	// given — two store instances sharing the same DB
	dbPath := filepath.Join(t.TempDir(), ".run", "outbox.db")
	var calls1, calls2 []deliverCall
	store1, err := session.NewSQLiteOutboxStore(dbPath, stubDeliverFn(&calls1))
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore 1: %v", err)
	}
	defer store1.Close()

	store2, err := session.NewSQLiteOutboxStore(dbPath, stubDeliverFn(&calls2))
	if err != nil {
		t.Fatalf("NewSQLiteOutboxStore 2: %v", err)
	}
	defer store2.Close()

	// when — concurrent Stage + Flush from both stores
	errs := make(chan error, 4)

	go func() {
		for i := 0; i < 10; i++ {
			name := fmt.Sprintf("s1-%03d.md", i)
			if err := store1.Stage(name, "report", "/repo/outbox1", []byte("data1")); err != nil {
				errs <- fmt.Errorf("store1 Stage: %w", err)
				return
			}
		}
		if _, err := store1.Flush(); err != nil {
			errs <- fmt.Errorf("store1 Flush: %w", err)
			return
		}
		errs <- nil
	}()

	go func() {
		for i := 0; i < 10; i++ {
			name := fmt.Sprintf("s2-%03d.md", i)
			if err := store2.Stage(name, "report", "/repo/outbox2", []byte("data2")); err != nil {
				errs <- fmt.Errorf("store2 Stage: %w", err)
				return
			}
		}
		if _, err := store2.Flush(); err != nil {
			errs <- fmt.Errorf("store2 Flush: %w", err)
			return
		}
		errs <- nil
	}()

	// then — no errors
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent error: %v", err)
		}
	}

	// Verify all items flushed
	pending1, _ := store1.FlushPending()
	if pending1 != 0 {
		t.Errorf("store1 FlushPending = %d, want 0", pending1)
	}
}
