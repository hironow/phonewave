package phonewave

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DeliveryLog writes append-only delivery records to .phonewave/delivery.log.
type DeliveryLog struct {
	file *os.File
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

func (l *DeliveryLog) write(action, details string) {
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Fprintf(l.file, "%s %-9s %s\n", ts, action, details)
}

// SaveToErrorQueue copies a failed D-Mail to .phonewave/errors/ for later retry.
func SaveToErrorQueue(stateDir, sourcePath string, data []byte) error {
	errorsDir := filepath.Join(stateDir, "errors")
	if err := os.MkdirAll(errorsDir, 0755); err != nil {
		return err
	}

	ts := time.Now().UTC().Format("2006-01-02T1504")
	baseName := filepath.Base(sourcePath)
	errorFile := filepath.Join(errorsDir, fmt.Sprintf("%s-%s", ts, baseName))

	return os.WriteFile(errorFile, data, 0644)
}
