package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	phonewave "github.com/hironow/phonewave"
)

// Compile-time check that DeliveryLog implements phonewave.DeliveryLogger.
var _ phonewave.DeliveryLogger = (*DeliveryLog)(nil)

// DeliveryLog writes append-only delivery records to .phonewave/delivery.log.
// All methods are safe for concurrent use.
type DeliveryLog struct {
	file *os.File
	mu   sync.Mutex
}

// NewDeliveryLog opens (or creates) the delivery log file.
func NewDeliveryLog(stateDir string) (*DeliveryLog, error) {
	logPath := filepath.Join(stateDir, "delivery.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open delivery log: %w", err)
	}
	return &DeliveryLog{file: f}, nil
}

// Close closes the log file.
func (l *DeliveryLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// Delivered records a successful delivery.
func (l *DeliveryLog) Delivered(kind, from, to string) {
	l.write("DELIVERED", fmt.Sprintf("kind=%s from=%s to=%s", kind, from, to))
}

// Removed records a source file removal after delivery.
func (l *DeliveryLog) Removed(from string) {
	l.write("REMOVED", fmt.Sprintf("from=%s", from))
}

// Failed records a delivery failure.
func (l *DeliveryLog) Failed(kind, from, reason string) {
	l.write("FAILED", fmt.Sprintf("kind=%s from=%s reason=%s", kind, from, reason))
}

// Retried records a successful retry delivery.
func (l *DeliveryLog) Retried(kind, from, to string) {
	l.write("RETRIED", fmt.Sprintf("kind=%s from=%s to=%s", kind, from, to))
}

func (l *DeliveryLog) write(action, details string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Fprintf(l.file, "%s %-9s %s\n", ts, action, details)
}
