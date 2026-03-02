package session_test

import (
	"fmt"
	"testing"
	"time"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/session"
)

func testErrorStore(t *testing.T) *session.SQLiteErrorStore {
	t.Helper()
	store, err := session.NewSQLiteErrorStore(t.TempDir())
	if err != nil {
		t.Fatalf("create error store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSessionErrorStore_RecordAndPending(t *testing.T) {
	// given
	store := testErrorStore(t)
	meta := phonewave.ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "test.md",
		Attempts:     1,
		Error:        "no route",
		Timestamp:    time.Now().UTC(),
	}

	// when
	if err := store.RecordError("test.md", []byte("hello"), meta); err != nil {
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
	if entries[0].Name != "test.md" {
		t.Errorf("name: got %q, want %q", entries[0].Name, "test.md")
	}
}

func TestSessionErrorStore_RecordIdempotent(t *testing.T) {
	// given
	store := testErrorStore(t)
	meta := phonewave.ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "dup.md",
		Attempts:     1,
		Error:        "error1",
		Timestamp:    time.Now().UTC(),
	}

	// when: record twice
	store.RecordError("dup.md", []byte("first"), meta)
	store.RecordError("dup.md", []byte("second"), meta)

	entries, err := store.PendingErrors(3)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}

	// then: first wins
	if len(entries) != 1 {
		t.Fatalf("pending count: got %d, want 1", len(entries))
	}
	if string(entries[0].Data) != "first" {
		t.Errorf("data: got %q, want %q", string(entries[0].Data), "first")
	}
}

func TestSessionErrorStore_IncrementRetry_Transaction(t *testing.T) {
	// given
	store := testErrorStore(t)
	meta := phonewave.ErrorMetadata{
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
	if entries[0].RetryCount != 1 {
		t.Errorf("retry_count: got %d, want 1", entries[0].RetryCount)
	}
	if entries[0].ErrorMessage != "retry error" {
		t.Errorf("error_message: got %q, want %q", entries[0].ErrorMessage, "retry error")
	}
}

func TestSessionErrorStore_MarkResolved_Transaction(t *testing.T) {
	// given
	store := testErrorStore(t)
	meta := phonewave.ErrorMetadata{
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
		t.Errorf("pending count: got %d, want 0", len(entries))
	}
}

func TestSessionErrorStore_ConcurrentAccess(t *testing.T) {
	// given: two store connections to same DB
	dir := t.TempDir()
	storeA, err := session.NewSQLiteErrorStore(dir)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	t.Cleanup(func() { storeA.Close() })

	storeB, err := session.NewSQLiteErrorStore(dir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	t.Cleanup(func() { storeB.Close() })

	const itemsPerStore = 10

	// when: concurrent writes
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	go func() {
		for i := range itemsPerStore {
			name := fmt.Sprintf("a-%03d.md", i)
			meta := phonewave.ErrorMetadata{
				SourceOutbox: "/tmp/outbox-a", Kind: "report",
				OriginalName: name, Attempts: 1, Error: "error-a",
				Timestamp: time.Now().UTC(),
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
			meta := phonewave.ErrorMetadata{
				SourceOutbox: "/tmp/outbox-b", Kind: "feedback",
				OriginalName: name, Attempts: 1, Error: "error-b",
				Timestamp: time.Now().UTC(),
			}
			if err := storeB.RecordError(name, []byte("data-b"), meta); err != nil {
				errB <- err
				return
			}
		}
		errB <- nil
	}()

	// then
	if e := <-errA; e != nil {
		t.Fatalf("store A: %v", e)
	}
	if e := <-errB; e != nil {
		t.Fatalf("store B: %v", e)
	}

	entries, err := storeA.PendingErrors(10)
	if err != nil {
		t.Fatalf("PendingErrors: %v", err)
	}
	if len(entries) != 2*itemsPerStore {
		t.Errorf("pending count: got %d, want %d", len(entries), 2*itemsPerStore)
	}
}
