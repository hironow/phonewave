package session_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hironow/phonewave/internal/session"
)

func TestDeliveryLog_Append(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := session.NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatalf("NewDeliveryLog: %v", err)
	}
	defer log.Close()

	// when
	log.Delivered("design-feedback", "/repo/.gate/outbox/feedback-001.md", "/repo/.siren/inbox/feedback-001.md")
	log.Delivered("design-feedback", "/repo/.gate/outbox/feedback-001.md", "/repo/.expedition/inbox/feedback-001.md")
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
	if !strings.Contains(lines[0], "kind=design-feedback") {
		t.Errorf("line 0 should contain kind=design-feedback: %s", lines[0])
	}
	if !strings.Contains(lines[2], "REMOVED") {
		t.Errorf("line 2 should contain REMOVED: %s", lines[2])
	}
}

func TestDeliveryLog_Failed(t *testing.T) {
	// given
	stateDir := t.TempDir()
	log, err := session.NewDeliveryLog(stateDir)
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
	log, err := session.NewDeliveryLog(stateDir)
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
	log, err := session.NewDeliveryLog(stateDir)
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
			log.Delivered("design-feedback", fmt.Sprintf("/outbox/msg-%d.md", n), "/inbox/msg.md")
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

// TestDeliveryLog_AppendAcrossRestarts verifies log is append-only across reopens.
func TestDeliveryLog_AppendAcrossRestarts(t *testing.T) {
	stateDir := t.TempDir()

	// First session
	log1, err := session.NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	log1.Delivered("specification", "/outbox/spec-001.md", "/inbox/spec-001.md")
	log1.Close()

	// Second session (simulates daemon restart)
	log2, err := session.NewDeliveryLog(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	log2.Delivered("design-feedback", "/outbox/fb-001.md", "/inbox/fb-001.md")
	log2.Close()

	// then — log should contain entries from both sessions
	data, err := os.ReadFile(filepath.Join(stateDir, "delivery.log"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	lines := 0
	for _, c := range content {
		if c == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("log lines = %d, want 2 (across restarts)", lines)
	}
}

// TestRace_DeliveryLog_ConcurrentWrite verifies the DeliveryLog mutex
// protects concurrent writes.
func TestRace_DeliveryLog_ConcurrentWrite(t *testing.T) {
	dir := t.TempDir()
	log, err := session.NewDeliveryLog(dir)
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
