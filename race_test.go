package phonewave

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestRace_ErrorStore_ConcurrentRecordAndRead verifies that concurrent
// RecordError and PendingErrors calls do not trigger the race detector.
func TestRace_ErrorStore_ConcurrentRecordAndRead(t *testing.T) {
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	var wg sync.WaitGroup
	const workers = 10

	for i := range workers {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("race-%03d.md", id)
			meta := ErrorMetadata{
				SourceOutbox: "/tmp/outbox",
				Kind:         "report",
				OriginalName: name,
				Attempts:     1,
				Error:        "race test",
				Timestamp:    time.Now().UTC(),
			}
			store.RecordError(name, []byte("data"), meta)
		}(i)
		go func() {
			defer wg.Done()
			store.PendingErrors(10)
		}()
	}
	wg.Wait()
}

// TestRace_ErrorStore_ConcurrentIncrementAndResolve verifies that
// concurrent IncrementRetry and MarkResolved do not race.
func TestRace_ErrorStore_ConcurrentIncrementAndResolve(t *testing.T) {
	stateDir := t.TempDir()
	store := testErrorStore(t, stateDir)

	// Seed entries
	for i := range 20 {
		name := fmt.Sprintf("entry-%03d.md", i)
		meta := ErrorMetadata{
			SourceOutbox: "/tmp/outbox",
			Kind:         "report",
			OriginalName: name,
			Attempts:     1,
			Error:        "initial",
			Timestamp:    time.Now().UTC(),
		}
		store.RecordError(name, []byte("data"), meta)
	}

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("entry-%03d.md", id)
			if id%2 == 0 {
				store.IncrementRetry(name, "retry")
			} else {
				store.MarkResolved(name)
			}
		}(i)
	}
	wg.Wait()
}

// TestRace_DeliveryLog_ConcurrentWrite verifies the DeliveryLog mutex
// protects concurrent writes.
func TestRace_DeliveryLog_ConcurrentWrite(t *testing.T) {
	dir := t.TempDir()
	log, err := NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	t.Cleanup(func() { log.Close() })

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("item-%03d.md", id)
			if id%2 == 0 {
				log.Delivered("report", name, "/tmp/dst")
			} else {
				log.Failed("report", name, "error")
			}
		}(i)
	}
	wg.Wait()
}

// TestRace_Logger_ConcurrentLog verifies that Logger's mutex protects
// concurrent log writes.
func TestRace_Logger_ConcurrentLog(t *testing.T) {
	logger := NewLogger(nil, false)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info("concurrent log %d", id)
		}(i)
	}
	wg.Wait()
}
