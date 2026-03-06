package platform

// white-box-reason: platform internals: tests unexported logger constructor and thread safety

import (
	"sync"
	"testing"
)

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
