package session_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	phonewave "github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/session"
)

func testErrorStore(t *testing.T) *session.SQLiteErrorStore {
	t.Helper()
	dir := t.TempDir()
	store, err := session.NewSQLiteErrorStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteErrorStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteErrorStore_PragmaSynchronousNormal(t *testing.T) {
	// given
	store := testErrorStore(t)

	// when: query PRAGMA on the store's own connection
	var synchronous string
	if err := store.DBForTest().QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("query PRAGMA synchronous: %v", err)
	}

	// then: synchronous = 1 (NORMAL)
	if synchronous != "1" {
		t.Errorf("PRAGMA synchronous: got %q, want %q (NORMAL)", synchronous, "1")
	}
}

func TestSQLiteErrorStore_RecordAndList(t *testing.T) {
	// given
	store := testErrorStore(t)
	entry := phonewave.RetryEntry{
		Name:         "20260226T120000-report-test.md",
		SourceOutbox: "/tmp/repo/.siren/outbox",
		Kind:         "report",
		OriginalName: "test.md",
		Data:         []byte("hello"),
		Attempts:     1,
		Error:        "no matching route",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// when
	err := store.RecordFailure(entry)
	if err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}

	// then
	entries, err := store.ListPending(10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ListPending count: got %d, want 1", len(entries))
	}
	got := entries[0]
	if got.Name != entry.Name {
		t.Errorf("Name: got %q, want %q", got.Name, entry.Name)
	}
	if got.Kind != "report" {
		t.Errorf("Kind: got %q, want %q", got.Kind, "report")
	}
	if string(got.Data) != "hello" {
		t.Errorf("Data: got %q, want %q", string(got.Data), "hello")
	}
	if got.Attempts != 1 {
		t.Errorf("Attempts: got %d, want 1", got.Attempts)
	}
}

func TestSQLiteErrorStore_RecordIdempotent(t *testing.T) {
	// given
	store := testErrorStore(t)
	entry := phonewave.RetryEntry{
		Name:         "dup-entry",
		SourceOutbox: "/tmp/outbox",
		Kind:         "specification",
		OriginalName: "dup.md",
		Data:         []byte("first"),
		Attempts:     1,
		Error:        "err1",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// when: record same name twice
	if err := store.RecordFailure(entry); err != nil {
		t.Fatalf("RecordFailure 1: %v", err)
	}
	entry.Data = []byte("second")
	if err := store.RecordFailure(entry); err != nil {
		t.Fatalf("RecordFailure 2: %v", err)
	}

	// then: only one entry, with first data (INSERT OR IGNORE)
	entries, err := store.ListPending(10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("count: got %d, want 1", len(entries))
	}
	if string(entries[0].Data) != "first" {
		t.Errorf("Data: got %q, want %q", string(entries[0].Data), "first")
	}
}

func TestSQLiteErrorStore_MarkRetried(t *testing.T) {
	// given
	store := testErrorStore(t)
	entry := phonewave.RetryEntry{
		Name:         "retry-me",
		SourceOutbox: "/tmp/outbox",
		Kind:         "feedback",
		OriginalName: "retry.md",
		Data:         []byte("data"),
		Attempts:     1,
		Error:        "first error",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	store.RecordFailure(entry)

	// when
	err := store.MarkRetried("retry-me", "second error")
	if err != nil {
		t.Fatalf("MarkRetried: %v", err)
	}

	// then
	entries, err := store.ListPending(10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("count: got %d, want 1", len(entries))
	}
	if entries[0].Attempts != 2 {
		t.Errorf("Attempts: got %d, want 2", entries[0].Attempts)
	}
	if entries[0].Error != "second error" {
		t.Errorf("Error: got %q, want %q", entries[0].Error, "second error")
	}
}

func TestSQLiteErrorStore_RemoveEntry(t *testing.T) {
	// given
	store := testErrorStore(t)
	entry := phonewave.RetryEntry{
		Name:         "to-remove",
		SourceOutbox: "/tmp/outbox",
		Kind:         "convergence",
		OriginalName: "remove.md",
		Data:         []byte("data"),
		Attempts:     1,
		Error:        "err",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	store.RecordFailure(entry)

	// when
	err := store.RemoveEntry("to-remove")
	if err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}

	// then
	entries, err := store.ListPending(10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("count: got %d, want 0", len(entries))
	}
}

func TestSQLiteErrorStore_ListPendingRespectsMaxRetries(t *testing.T) {
	// given: one entry with attempts=5
	store := testErrorStore(t)
	entry := phonewave.RetryEntry{
		Name:         "maxed-out",
		SourceOutbox: "/tmp/outbox",
		Kind:         "report",
		OriginalName: "max.md",
		Data:         []byte("data"),
		Attempts:     5,
		Error:        "persistent error",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	store.RecordFailure(entry)

	// when: list with maxRetries=5 (entry.Attempts >= maxRetries)
	entries, err := store.ListPending(5)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}

	// then: excluded (attempts >= maxRetries)
	if len(entries) != 0 {
		t.Errorf("count: got %d, want 0", len(entries))
	}

	// when: list with maxRetries=6
	entries, err = store.ListPending(6)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}

	// then: included (attempts < maxRetries)
	if len(entries) != 1 {
		t.Errorf("count: got %d, want 1", len(entries))
	}
}

func TestSQLiteErrorStore_ListPendingOrderByCreatedAt(t *testing.T) {
	// given
	store := testErrorStore(t)
	now := time.Now().UTC()

	for i := range 3 {
		entry := phonewave.RetryEntry{
			Name:         fmt.Sprintf("entry-%d", i),
			SourceOutbox: "/tmp/outbox",
			Kind:         "report",
			OriginalName: fmt.Sprintf("e%d.md", i),
			Data:         []byte(fmt.Sprintf("data-%d", i)),
			Attempts:     1,
			Error:        "err",
			CreatedAt:    now.Add(time.Duration(2-i) * time.Second), // reverse order: 2, 1, 0
			UpdatedAt:    now,
		}
		store.RecordFailure(entry)
	}

	// when
	entries, err := store.ListPending(10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}

	// then: ordered by created_at ascending (oldest first)
	if len(entries) != 3 {
		t.Fatalf("count: got %d, want 3", len(entries))
	}
	if entries[0].Name != "entry-2" {
		t.Errorf("first entry: got %q, want %q", entries[0].Name, "entry-2")
	}
	if entries[2].Name != "entry-0" {
		t.Errorf("last entry: got %q, want %q", entries[2].Name, "entry-0")
	}
}

func TestSQLiteErrorStore_RecordFailure_ConcurrentWrites(t *testing.T) {
	// given: single store with 20 concurrent goroutines
	store := testErrorStore(t)

	const writers = 20
	var wg sync.WaitGroup
	errs := make(chan error, writers)

	// when: 20 goroutines write unique entries concurrently
	wg.Add(writers)
	for i := range writers {
		go func(idx int) {
			defer wg.Done()
			entry := phonewave.RetryEntry{
				Name:         fmt.Sprintf("concurrent-%03d", idx),
				SourceOutbox: "/tmp/outbox",
				Kind:         "report",
				OriginalName: fmt.Sprintf("c-%03d.md", idx),
				Data:         []byte(fmt.Sprintf("data-%d", idx)),
				Attempts:     1,
				Error:        "test error",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}
			errs <- store.RecordFailure(entry)
		}(i)
	}
	wg.Wait()
	close(errs)

	// then: all writes succeed
	for err := range errs {
		if err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}
	}

	// then: all 20 entries recorded
	entries, err := store.ListPending(10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(entries) != writers {
		t.Errorf("count: got %d, want %d", len(entries), writers)
	}
}

func TestSQLiteErrorStore_ConcurrentRecordAndList(t *testing.T) {
	// given: two stores sharing the same DB (simulating concurrent CLI processes)
	dir := t.TempDir()
	storeA, err := session.NewSQLiteErrorStore(dir)
	if err != nil {
		t.Fatalf("create store A: %v", err)
	}
	defer storeA.Close()

	storeB, err := session.NewSQLiteErrorStore(dir)
	if err != nil {
		t.Fatalf("create store B: %v", err)
	}
	defer storeB.Close()

	const itemsPerStore = 10

	// when: both stores Record concurrently
	var wg sync.WaitGroup
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range itemsPerStore {
			entry := phonewave.RetryEntry{
				Name:         fmt.Sprintf("a-%03d", i),
				SourceOutbox: "/tmp/outbox",
				Kind:         "report",
				OriginalName: fmt.Sprintf("a-%03d.md", i),
				Data:         []byte(fmt.Sprintf("data-a-%d", i)),
				Attempts:     1,
				Error:        "err",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}
			if err := storeA.RecordFailure(entry); err != nil {
				errA <- err
				return
			}
		}
		errA <- nil
	}()
	go func() {
		defer wg.Done()
		for i := range itemsPerStore {
			entry := phonewave.RetryEntry{
				Name:         fmt.Sprintf("b-%03d", i),
				SourceOutbox: "/tmp/outbox",
				Kind:         "specification",
				OriginalName: fmt.Sprintf("b-%03d.md", i),
				Data:         []byte(fmt.Sprintf("data-b-%d", i)),
				Attempts:     1,
				Error:        "err",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}
			if err := storeB.RecordFailure(entry); err != nil {
				errB <- err
				return
			}
		}
		errB <- nil
	}()
	wg.Wait()

	// then: no errors
	if e := <-errA; e != nil {
		t.Fatalf("store A error: %v", e)
	}
	if e := <-errB; e != nil {
		t.Fatalf("store B error: %v", e)
	}

	// then: all 20 entries visible from either store
	entries, err := storeA.ListPending(10)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(entries) != 2*itemsPerStore {
		t.Errorf("total entries: got %d, want %d", len(entries), 2*itemsPerStore)
	}
}
