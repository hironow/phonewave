package phonewave

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testErrorStore(t *testing.T, stateDir string) *SQLiteErrorStore {
	t.Helper()
	store, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatalf("create error store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteErrorStore_RecordAndPending(t *testing.T) {
	// given
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	meta := ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "test-mail.md",
		Attempts:     1,
		Error:        "no route",
		Timestamp:    time.Now().UTC(),
	}

	// when
	if err := store.RecordError("test-mail.md", []byte("hello"), meta); err != nil {
		t.Fatalf("RecordError: %v", err)
	}

	entries, err := store.PendingErrors(3)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}

	// then
	if len(entries) != 1 {
		t.Fatalf("pending count: got %d, want 1", len(entries))
	}
	if entries[0].Name != "test-mail.md" {
		t.Errorf("name: got %q, want %q", entries[0].Name, "test-mail.md")
	}
	if string(entries[0].Data) != "hello" {
		t.Errorf("data: got %q, want %q", string(entries[0].Data), "hello")
	}
	if entries[0].Kind != "report" {
		t.Errorf("kind: got %q, want %q", entries[0].Kind, "report")
	}
	if entries[0].SourceOutbox != "/tmp/outbox" {
		t.Errorf("source_outbox: got %q, want %q", entries[0].SourceOutbox, "/tmp/outbox")
	}
	if entries[0].RetryCount != 0 {
		t.Errorf("retry_count: got %d, want 0", entries[0].RetryCount)
	}
}

func TestSQLiteErrorStore_RecordIdempotent(t *testing.T) {
	// given
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	meta := ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "dup.md",
		Attempts:     1,
		Error:        "error1",
		Timestamp:    time.Now().UTC(),
	}

	// when: record twice with same name
	store.RecordError("dup.md", []byte("first"), meta)
	store.RecordError("dup.md", []byte("second"), meta)

	entries, err := store.PendingErrors(3)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}

	// then: only first insert wins (INSERT OR IGNORE)
	if len(entries) != 1 {
		t.Fatalf("pending count: got %d, want 1", len(entries))
	}
	if string(entries[0].Data) != "first" {
		t.Errorf("data: got %q, want %q (first wins)", string(entries[0].Data), "first")
	}
}

func TestSQLiteErrorStore_IncrementRetry(t *testing.T) {
	// given
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	meta := ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "feedback",
		OriginalName: "fail.md",
		Attempts:     1,
		Error:        "initial error",
		Timestamp:    time.Now().UTC(),
	}
	store.RecordError("fail.md", []byte("data"), meta)

	// when
	if err := store.IncrementRetry("fail.md", "retry error"); err != nil {
		t.Fatalf("IncrementRetry: %v", err)
	}

	entries, err := store.PendingErrors(3)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}

	// then
	if len(entries) != 1 {
		t.Fatalf("pending count: got %d, want 1", len(entries))
	}
	if entries[0].RetryCount != 1 {
		t.Errorf("retry_count: got %d, want 1", entries[0].RetryCount)
	}
	if entries[0].ErrorMessage != "retry error" {
		t.Errorf("error_message: got %q, want %q", entries[0].ErrorMessage, "retry error")
	}
}

func TestSQLiteErrorStore_MaxRetries_DeadLetter(t *testing.T) {
	// given
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	meta := ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "dead.md",
		Attempts:     1,
		Error:        "error",
		Timestamp:    time.Now().UTC(),
	}
	store.RecordError("dead.md", []byte("data"), meta)

	// when: increment 3 times (maxRetryCount)
	for range 3 {
		store.IncrementRetry("dead.md", "still failing")
	}

	entries, err := store.PendingErrors(3)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}

	// then: no pending entries (dead-letter)
	if len(entries) != 0 {
		t.Errorf("pending count: got %d, want 0 (dead-letter)", len(entries))
	}
}

func TestSQLiteErrorStore_MarkResolved(t *testing.T) {
	// given
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	meta := ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "resolved.md",
		Attempts:     1,
		Error:        "error",
		Timestamp:    time.Now().UTC(),
	}
	store.RecordError("resolved.md", []byte("data"), meta)

	// when
	if err := store.MarkResolved("resolved.md"); err != nil {
		t.Fatalf("MarkResolved: %v", err)
	}

	entries, err := store.PendingErrors(3)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}

	// then
	if len(entries) != 0 {
		t.Errorf("pending count: got %d, want 0 (resolved)", len(entries))
	}
}

func TestSQLiteErrorStore_FilePermission(t *testing.T) {
	if os.Getenv("CI") != "" && strings.Contains(strings.ToLower(os.Getenv("RUNNER_OS")), "windows") {
		t.Skip("NTFS does not support Unix file permissions")
	}

	// given
	stateDir := t.TempDir()
	_ = testErrorStore(t, stateDir)

	// when
	dbPath := filepath.Join(stateDir, "errors.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}

	// then: permission should be 0o600 (owner read/write only)
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("db permission: got %o, want %o", perm, 0o600)
	}
}

func TestSQLiteErrorStore_SchemaConsistency(t *testing.T) {
	// given
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	// when: query schema info
	var tableName string
	err := store.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='errors'`,
	).Scan(&tableName)
	if err != nil {
		t.Fatalf("query schema: %v", err)
	}

	// then: table exists
	if tableName != "errors" {
		t.Errorf("table name: got %q, want %q", tableName, "errors")
	}

	// Verify columns exist
	rows, err := store.db.Query(`PRAGMA table_info(errors)`)
	if err != nil {
		t.Fatalf("query table_info: %v", err)
	}
	defer rows.Close()

	expectedCols := map[string]bool{
		"name":          false,
		"data":          false,
		"source_outbox": false,
		"kind":          false,
		"error_message": false,
		"retry_count":   false,
		"resolved":      false,
	}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		if _, ok := expectedCols[name]; ok {
			expectedCols[name] = true
		}
	}

	for col, found := range expectedCols {
		if !found {
			t.Errorf("missing column: %s", col)
		}
	}
}

func TestSQLiteErrorStore_ConcurrentRecordAndPending(t *testing.T) {
	// given: two store connections to the same DB (simulating 2 CLI processes)
	stateDir := t.TempDir()
	storeA := testErrorStore(t, stateDir)
	storeB, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	t.Cleanup(func() { storeB.Close() })

	const itemsPerStore = 10

	// when: both stores record concurrently
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	go func() {
		for i := range itemsPerStore {
			name := fmt.Sprintf("a-%03d.md", i)
			meta := ErrorMetadata{
				SourceOutbox: "/tmp/outbox-a",
				Kind:         "report",
				OriginalName: name,
				Attempts:     1,
				Error:        "error-a",
				Timestamp:    time.Now().UTC(),
			}
			if err := storeA.RecordError(name, []byte("data-a"), meta); err != nil {
				errA <- err
				return
			}
		}
		errA <- nil
	}()
	go func() {
		for i := range itemsPerStore {
			name := fmt.Sprintf("b-%03d.md", i)
			meta := ErrorMetadata{
				SourceOutbox: "/tmp/outbox-b",
				Kind:         "feedback",
				OriginalName: name,
				Attempts:     1,
				Error:        "error-b",
				Timestamp:    time.Now().UTC(),
			}
			if err := storeB.RecordError(name, []byte("data-b"), meta); err != nil {
				errB <- err
				return
			}
		}
		errB <- nil
	}()

	// then: no errors from either store
	if e := <-errA; e != nil {
		t.Fatalf("store A error: %v", e)
	}
	if e := <-errB; e != nil {
		t.Fatalf("store B error: %v", e)
	}

	// then: all 20 entries are pending
	entries, err := storeA.PendingErrors(10)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}
	if len(entries) != 2*itemsPerStore {
		t.Errorf("pending count: got %d, want %d", len(entries), 2*itemsPerStore)
	}
}

func TestSQLiteErrorStore_ConcurrentIncrementAndResolve(t *testing.T) {
	// given: a shared store with multiple records
	stateDir := t.TempDir()
	storeA := testErrorStore(t, stateDir)
	storeB, err := NewSQLiteErrorStore(stateDir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	t.Cleanup(func() { storeB.Close() })

	meta := ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "shared.md",
		Attempts:     1,
		Error:        "error",
		Timestamp:    time.Now().UTC(),
	}
	storeA.RecordError("increment.md", []byte("data"), meta)
	storeA.RecordError("resolve.md", []byte("data"), meta)

	// when: one store increments retry, another resolves — concurrently
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	go func() {
		errA <- storeA.IncrementRetry("increment.md", "retry-1")
	}()
	go func() {
		errB <- storeB.MarkResolved("resolve.md")
	}()

	// then: no errors
	if e := <-errA; e != nil {
		t.Fatalf("IncrementRetry error: %v", e)
	}
	if e := <-errB; e != nil {
		t.Fatalf("MarkResolved error: %v", e)
	}

	// verify states
	entries, err := storeA.PendingErrors(10)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("pending count: got %d, want 1 (resolved entry excluded)", len(entries))
	}
	if entries[0].Name != "increment.md" {
		t.Errorf("pending name: got %q, want %q", entries[0].Name, "increment.md")
	}
	if entries[0].RetryCount != 1 {
		t.Errorf("retry_count: got %d, want 1", entries[0].RetryCount)
	}
}

func TestSQLiteErrorStore_PragmaSynchronousNormal(t *testing.T) {
	// given
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	// when
	var synchronous string
	if err := store.db.QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("query PRAGMA synchronous: %v", err)
	}

	// then: synchronous = 1 (NORMAL)
	if synchronous != "1" {
		t.Errorf("PRAGMA synchronous: got %q, want %q (NORMAL)", synchronous, "1")
	}
}
