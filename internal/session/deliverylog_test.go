package session

import (
	"fmt"
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
	log.Delivered("feedback", "/repo/.gate/outbox/feedback-001.md", "/repo/.siren/inbox/feedback-001.md")
	log.Delivered("feedback", "/repo/.gate/outbox/feedback-001.md", "/repo/.expedition/inbox/feedback-001.md")
	log.Removed("/repo/.gate/outbox/feedback-001.md")

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

func TestDeliveryLog_Retried(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	defer log.Close()

	// when
	log.Retried("specification", "/repo/.siren/outbox/spec-001.md", "/repo/.expedition/inbox/spec-001.md")

	// then
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "RETRIED") {
		t.Error("log should contain RETRIED")
	}
	if !strings.Contains(content, "kind=specification") {
		t.Error("log should contain kind=specification")
	}
}

func TestDeliveryLog_CloseIsConcurrencySafe(t *testing.T) {
	// given — a delivery log with concurrent writers
	stateDir := t.TempDir()
	log, err := NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}

	// when — write and close concurrently to expose race on l.file
	start := make(chan struct{})
	done := make(chan struct{}, 11)

	for i := range 10 {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			<-start
			log.Delivered("feedback", fmt.Sprintf("/outbox/msg-%d.md", n), "/inbox/msg.md")
		}(i)
	}
	go func() {
		defer func() { done <- struct{}{} }()
		<-start
		log.Close()
	}()

	// Release all goroutines at once to maximize contention
	close(start)
	for range 11 {
		<-done
	}

	// then — no race detected (verified by -race flag)
}
