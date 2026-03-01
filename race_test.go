package phonewave

import (
	"fmt"
	"sync"
	"testing"
)

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
