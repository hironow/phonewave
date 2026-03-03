package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"gopkg.in/yaml.v3"
)

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

// SaveToErrorQueue saves a failed D-Mail to .phonewave/errors/ with a .err sidecar.
// Filename format: {timestamp}-{kind}-{original_name}
func SaveToErrorQueue(stateDir string, meta domain.ErrorMetadata, data []byte) error {
	errorsDir := filepath.Join(stateDir, "errors")
	if err := os.MkdirAll(errorsDir, 0755); err != nil {
		return err
	}

	ts := meta.Timestamp.Format("2006-01-02T150405.000000000")
	// Sanitize Kind and OriginalName to prevent path traversal
	safeKind := filepath.Base(meta.Kind)
	safeName := filepath.Base(meta.OriginalName)
	errorFile := filepath.Join(errorsDir, fmt.Sprintf("%s-%s-%s", ts, safeKind, safeName))

	if err := os.WriteFile(errorFile, data, 0644); err != nil {
		return fmt.Errorf("write error file: %w", err)
	}

	sidecarData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal error metadata: %w", err)
	}

	sidecarPath := errorFile + ".err"
	if err := os.WriteFile(sidecarPath, sidecarData, 0644); err != nil {
		return fmt.Errorf("write error sidecar: %w", err)
	}

	return nil
}

// UpdateErrorMetadata increments the attempts counter and updates the error message
// in an existing .err sidecar file.
func UpdateErrorMetadata(sidecarPath string, newError string) error {
	meta, err := LoadErrorMetadata(sidecarPath)
	if err != nil {
		return err
	}

	meta.Attempts++
	meta.Error = newError
	meta.Timestamp = time.Now().UTC()

	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal error metadata: %w", err)
	}

	if err := os.WriteFile(sidecarPath, data, 0644); err != nil {
		return fmt.Errorf("write error sidecar: %w", err)
	}
	return nil
}

// RemoveErrorEntry removes a D-Mail file and its .err sidecar from the error queue.
func RemoveErrorEntry(dmailPath string) error {
	if err := os.Remove(dmailPath); err != nil {
		return fmt.Errorf("remove error entry: %w", err)
	}
	sidecarPath := dmailPath + ".err"
	if err := os.Remove(sidecarPath); err != nil {
		return fmt.Errorf("remove error sidecar: %w", err)
	}
	return nil
}

// LoadErrorMetadata reads and parses a .err sidecar file.
func LoadErrorMetadata(sidecarPath string) (*domain.ErrorMetadata, error) {
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return nil, fmt.Errorf("read error sidecar: %w", err)
	}

	var meta domain.ErrorMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse error sidecar: %w", err)
	}
	return &meta, nil
}
