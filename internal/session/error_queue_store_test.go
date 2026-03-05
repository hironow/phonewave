package session_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase/port"
	"github.com/hironow/phonewave/internal/session"
)

func testErrorQueueStore(t *testing.T) *session.SQLiteErrorQueueStore {
	t.Helper()
	store, err := session.NewSQLiteErrorQueueStore(t.TempDir())
	if err != nil {
		t.Fatalf("create error queue store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func testMeta(name string) domain.ErrorMetadata {
	return domain.ErrorMetadata{
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: name,
		Attempts:     1,
		Error:        "delivery failed",
		Timestamp:    time.Now().UTC(),
	}
}

func TestErrorQueueStore_EnqueueAndClaim(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	meta := testMeta("test.md")

	// when
	if err := store.Enqueue("test.md", []byte("hello"), meta); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	entries, err := store.ClaimPendingRetries("daemon-1", 3)
	if err != nil {
		t.Fatalf("ClaimPendingRetries: %v", err)
	}

	// then
	if len(entries) != 1 {
		t.Fatalf("claimed count: got %d, want 1", len(entries))
	}
	if entries[0].Name != "test.md" {
		t.Errorf("name: got %q, want %q", entries[0].Name, "test.md")
	}
	if string(entries[0].Data) != "hello" {
		t.Errorf("data: got %q, want %q", string(entries[0].Data), "hello")
	}
}

func TestErrorQueueStore_EnqueueIdempotent(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	meta := testMeta("dup.md")

	// when: enqueue twice
	store.Enqueue("dup.md", []byte("first"), meta)
	store.Enqueue("dup.md", []byte("second"), meta)

	entries, err := store.ClaimPendingRetries("daemon-1", 3)
	if err != nil {
		t.Fatalf("ClaimPendingRetries: %v", err)
	}

	// then: first wins
	if len(entries) != 1 {
		t.Fatalf("claimed count: got %d, want 1", len(entries))
	}
	if string(entries[0].Data) != "first" {
		t.Errorf("data: got %q, want %q", string(entries[0].Data), "first")
	}
}

func TestErrorQueueStore_ClaimExclusive(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	store.Enqueue("item.md", []byte("data"), testMeta("item.md"))

	// when: daemon-1 claims
	entries1, err := store.ClaimPendingRetries("daemon-1", 3)
	if err != nil {
		t.Fatalf("ClaimPendingRetries daemon-1: %v", err)
	}
	// daemon-2 tries to claim the same items
	entries2, err := store.ClaimPendingRetries("daemon-2", 3)
	if err != nil {
		t.Fatalf("ClaimPendingRetries daemon-2: %v", err)
	}

	// then: daemon-1 got it, daemon-2 gets nothing
	if len(entries1) != 1 {
		t.Errorf("daemon-1 claimed: got %d, want 1", len(entries1))
	}
	if len(entries2) != 0 {
		t.Errorf("daemon-2 claimed: got %d, want 0", len(entries2))
	}
}

func TestErrorQueueStore_PendingCount(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	store.Enqueue("a.md", []byte("a"), testMeta("a.md"))
	store.Enqueue("b.md", []byte("b"), testMeta("b.md"))

	// when
	count, err := store.PendingCount(3)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}

	// then
	if count != 2 {
		t.Errorf("pending count: got %d, want 2", count)
	}
}

func TestErrorQueueStore_IncrementRetry(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	store.Enqueue("fail.md", []byte("data"), testMeta("fail.md"))

	// when
	if err := store.IncrementRetry("fail.md", "retry error"); err != nil {
		t.Fatalf("IncrementRetry: %v", err)
	}

	entries, err := store.ClaimPendingRetries("daemon-1", 3)
	if err != nil {
		t.Fatalf("ClaimPendingRetries: %v", err)
	}

	// then: Enqueue sets retry_count=1 (from meta.Attempts), IncrementRetry adds 1 → 2
	if entries[0].RetryCount != 2 {
		t.Errorf("retry_count: got %d, want 2", entries[0].RetryCount)
	}
	if entries[0].ErrorMessage != "retry error" {
		t.Errorf("error_message: got %q, want %q", entries[0].ErrorMessage, "retry error")
	}
}

func TestErrorQueueStore_MarkResolved(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	store.Enqueue("resolved.md", []byte("data"), testMeta("resolved.md"))

	// when
	if err := store.MarkResolved("resolved.md"); err != nil {
		t.Fatalf("MarkResolved: %v", err)
	}

	count, err := store.PendingCount(3)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}

	// then
	if count != 0 {
		t.Errorf("pending count: got %d, want 0", count)
	}
}

func TestErrorQueueStore_ImplementsInterface(t *testing.T) {
	var _ port.ErrorQueueStore = testErrorQueueStore(t)
}

func TestEnqueue_SetsRetryCountFromAttempts(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	meta := testMeta("retry.md")
	meta.Attempts = 3

	// when
	if err := store.Enqueue("retry.md", []byte("data"), meta); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	entries, err := store.ClaimPendingRetries("daemon-1", 10)
	if err != nil {
		t.Fatalf("ClaimPendingRetries: %v", err)
	}

	// then
	if len(entries) != 1 {
		t.Fatalf("claimed count: got %d, want 1", len(entries))
	}
	if entries[0].RetryCount != 3 {
		t.Errorf("retry_count: got %d, want 3 (matching meta.Attempts)", entries[0].RetryCount)
	}
}

func TestClaimPendingRetries_ReturnsOriginalName(t *testing.T) {
	// given
	store := testErrorQueueStore(t)
	meta := testMeta("original-file.md")

	// when
	if err := store.Enqueue("queued.md", []byte("data"), meta); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	entries, err := store.ClaimPendingRetries("daemon-1", 10)
	if err != nil {
		t.Fatalf("ClaimPendingRetries: %v", err)
	}

	// then
	if len(entries) != 1 {
		t.Fatalf("claimed count: got %d, want 1", len(entries))
	}
	if entries[0].OriginalName != "original-file.md" {
		t.Errorf("OriginalName: got %q, want %q", entries[0].OriginalName, "original-file.md")
	}
}

func TestErrorQueueStore_ConcurrentAccess(t *testing.T) {
	// given: two store connections to same DB
	dir := t.TempDir()
	storeA, err := session.NewSQLiteErrorQueueStore(dir)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	t.Cleanup(func() { storeA.Close() })

	storeB, err := session.NewSQLiteErrorQueueStore(dir)
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
			if err := storeA.Enqueue(name, []byte("data-a"), testMeta(name)); err != nil {
				errA <- err
				return
			}
		}
		errA <- nil
	}()
	go func() {
		for i := range itemsPerStore {
			name := fmt.Sprintf("b-%03d.md", i)
			if err := storeB.Enqueue(name, []byte("data-b"), testMeta(name)); err != nil {
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

	count, err := storeA.PendingCount(3)
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}
	if count != 2*itemsPerStore {
		t.Errorf("pending count: got %d, want %d", count, 2*itemsPerStore)
	}
}
