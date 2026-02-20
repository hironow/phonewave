package phonewave

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryLog_Append(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	defer log.Close()

	// when
	log.Delivered("feedback", "/repo/.divergence/outbox/feedback-001.md", "/repo/.siren/inbox/feedback-001.md")
	log.Delivered("feedback", "/repo/.divergence/outbox/feedback-001.md", "/repo/.expedition/inbox/feedback-001.md")
	log.Removed("/repo/.divergence/outbox/feedback-001.md")

	// then — read the log file
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("log lines = %d, want 3", len(lines))
	}

	if !strings.Contains(lines[0], "DELIVERED") {
		t.Errorf("line 0 should contain DELIVERED: %s", lines[0])
	}
	if !strings.Contains(lines[0], "kind=feedback") {
		t.Errorf("line 0 should contain kind=feedback: %s", lines[0])
	}
	if !strings.Contains(lines[2], "REMOVED") {
		t.Errorf("line 2 should contain REMOVED: %s", lines[2])
	}
}

func TestDeliveryLog_Failed(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	defer log.Close()

	// when
	log.Failed("specification", "/repo/.siren/outbox/spec-001.md", "target inbox not found")

	// then
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "FAILED") {
		t.Error("log should contain FAILED")
	}
	if !strings.Contains(content, "kind=specification") {
		t.Error("log should contain kind=specification")
	}
}

func TestSaveToErrorQueue(t *testing.T) {
	// given
	stateDir := t.TempDir()
	errorsDir := filepath.Join(stateDir, "errors")
	if err := os.MkdirAll(errorsDir, 0755); err != nil {
		t.Fatal(err)
	}

	dmailContent := []byte(`---
name: spec-fail
kind: specification
---

# Failed spec
`)
	sourcePath := "/repo/.siren/outbox/spec-fail.md"

	// when
	err := SaveToErrorQueue(stateDir, sourcePath, dmailContent)

	// then
	if err != nil {
		t.Fatalf("SaveToErrorQueue: %v", err)
	}

	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("error queue entries = %d, want 1", len(entries))
	}
	if !strings.Contains(entries[0].Name(), "spec-fail.md") {
		t.Errorf("error file name = %q, want to contain 'spec-fail.md'", entries[0].Name())
	}
}
