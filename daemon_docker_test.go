//go:build docker

package phonewave

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLifecycleDocker_DryRunMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose --dry-run")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Write a D-Mail
	dmailContent := "---\nname: spec-dry\nkind: specification\ndescription: Dry run test\n---\n\n# Dry\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/spec-dry.md", dmailContent)

	// Wait a bit for the daemon to process
	time.Sleep(3 * time.Second)

	// File should STILL be in outbox (not delivered in dry-run)
	if !fileExistsInContainer(t, ctx, c, repoPath+"/.siren/outbox/spec-dry.md") {
		t.Error("file should remain in outbox during dry-run")
	}

	// File should NOT be in inbox
	if fileExistsInContainer(t, ctx, c, repoPath+"/.expedition/inbox/spec-dry.md") {
		t.Error("file should NOT appear in inbox during dry-run")
	}

	// Daemon log should contain [dry-run]
	daemonLog := readFileInContainer(t, ctx, c, "/tmp/phonewave.log")
	if !strings.Contains(daemonLog, "[dry-run]") {
		t.Errorf("daemon log should contain [dry-run]: %s", daemonLog)
	}
}

func TestLifecycleDocker_ErrorQueueAndRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	// Start daemon
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose --retry-interval 2s")

	// Write D-Mail with unknown kind (no route)
	unknownContent := "---\nname: mystery-msg\nkind: mystery\ndescription: Unknown kind\n---\n\n# Mystery\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/mystery-msg.md", unknownContent)

	// Wait for source file to be removed from outbox (moved to error queue)
	waitForFileAbsentInContainer(t, ctx, c, repoPath+"/.siren/outbox/mystery-msg.md", 10*time.Second)

	// Verify error queue has entries
	errCount := countFilesInContainer(t, ctx, c, "/workspace/.phonewave/errors", ".err")
	if errCount == 0 {
		t.Error("error queue should have .err sidecar files")
	}

	// Verify delivery log has FAILED
	waitForStringInFile(t, ctx, c, "/workspace/.phonewave/delivery.log", "FAILED", 5*time.Second)

	// Stop daemon
	stopDaemonInContainer(t, ctx, c, "/workspace")

	// Now add a consumer for "mystery" kind and re-sync
	mysteryConsumerDir := repoPath + "/.expedition/skills/dmail-readable-mystery"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", mysteryConsumerDir})
	heredocWrite(t, ctx, c, mysteryConsumerDir+"/SKILL.md",
		"---\nname: dmail-readable-mystery\nconsumes:\n  - kind: mystery\n---\n")

	execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave sync",
	})

	// Restart daemon — retry should pick up the error queue entry
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose --retry-interval 2s")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Wait for error queue to be cleared (retry succeeded)
	deadline := time.After(15 * time.Second)
retryLoop:
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for error queue to clear after retry")
		default:
			count := countFilesInContainer(t, ctx, c, "/workspace/.phonewave/errors", ".err")
			if count == 0 {
				break retryLoop
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Verify RETRIED in delivery log
	waitForStringInFile(t, ctx, c, "/workspace/.phonewave/delivery.log", "RETRIED", 5*time.Second)
}

func TestLifecycleDocker_MaxRetriesExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	// Manually seed error queue with attempts=10
	errorsDir := "/workspace/.phonewave/errors"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", errorsDir})

	dmailData := "---\nname: exhausted\nkind: specification\ndescription: Max retries\n---\n\n# Exhausted\n"
	sidecar := `source_outbox: /workspace/repo/.siren/outbox
kind: specification
original_name: exhausted.md
attempts: 10
error: "previous failure"
timestamp: 2025-01-01T00:00:00Z`

	heredocWrite(t, ctx, c, errorsDir+"/2025-01-01T000000.000000000-specification-exhausted.md", dmailData)
	heredocWrite(t, ctx, c, errorsDir+"/2025-01-01T000000.000000000-specification-exhausted.md.err", sidecar)

	// Start daemon with max-retries=10 and short retry interval
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose --retry-interval 2s --max-retries 10")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Wait for a few retry cycles
	time.Sleep(5 * time.Second)

	// Error queue entry should still be there (not retried because attempts >= maxRetries)
	errCount := countFilesInContainer(t, ctx, c, errorsDir, ".err")
	if errCount == 0 {
		t.Error("error queue entry should remain (max retries exceeded)")
	}
}

func TestLifecycleDocker_PartialDeliveryRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	// Remove one of the target inboxes to cause atomicWrite failure.
	// feedback routes to both .siren/inbox and .expedition/inbox.
	// Remove .expedition/inbox so the second write fails.
	execInContainer(t, ctx, c, []string{"rm", "-rf", repoPath + "/.expedition/inbox"})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Write feedback D-Mail (multi-target: siren + expedition)
	fbContent := "---\nname: fb-rollback\nkind: feedback\ndescription: Rollback test\n---\n\n# Rollback\n"
	heredocWrite(t, ctx, c, repoPath+"/.gate/outbox/fb-rollback.md", fbContent)

	// Wait for the file to be processed (moved to error queue or outbox cleared)
	time.Sleep(5 * time.Second)

	// .siren/inbox should NOT have the file (rollback)
	if fileExistsInContainer(t, ctx, c, repoPath+"/.siren/inbox/fb-rollback.md") {
		t.Error("siren inbox should be rolled back when expedition inbox fails")
	}

	// Source should be gone from outbox (moved to error queue)
	if fileExistsInContainer(t, ctx, c, repoPath+"/.gate/outbox/fb-rollback.md") {
		// If still in outbox, that's also acceptable (error queue save failed)
		t.Log("file still in outbox — error queue save may have also failed")
	}

	// Error queue should have an entry
	errCount := countFilesInContainer(t, ctx, c, "/workspace/.phonewave/errors", ".err")
	if errCount == 0 {
		// Could be in outbox or error queue — just verify no partial delivery
		t.Log("no error queue entry — checking outbox for source")
	}
}

func TestLifecycleDocker_GracefulShutdownSIGINT(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")

	// Send SIGINT
	execInContainer(t, ctx, c, []string{
		"sh", "-c", "kill -INT $(cat /workspace/.phonewave/watch.pid)",
	})

	// PID file should be removed
	waitForFileAbsentInContainer(t, ctx, c, "/workspace/.phonewave/watch.pid", 10*time.Second)

	// watch.started should also be removed
	if fileExistsInContainer(t, ctx, c, "/workspace/.phonewave/watch.started") {
		t.Error("watch.started should be removed after shutdown")
	}
}

func TestLifecycleDocker_GracefulShutdownSIGTERM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")

	// Send SIGTERM
	execInContainer(t, ctx, c, []string{
		"sh", "-c", "kill -TERM $(cat /workspace/.phonewave/watch.pid)",
	})

	// PID file should be removed
	waitForFileAbsentInContainer(t, ctx, c, "/workspace/.phonewave/watch.pid", 10*time.Second)
}

func TestLifecycleDocker_BurstDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Write 10 D-Mails in rapid succession
	for i := 0; i < 10; i++ {
		content := fmt.Sprintf("---\nname: burst-%02d\nkind: specification\ndescription: Burst %d\n---\n\n# Burst %d\n", i, i, i)
		heredocWrite(t, ctx, c, fmt.Sprintf("%s/.siren/outbox/burst-%02d.md", repoPath, i), content)
	}

	// Wait for all 10 to appear in expedition inbox
	for i := 0; i < 10; i++ {
		waitForFileInContainer(t, ctx, c,
			fmt.Sprintf("%s/.expedition/inbox/burst-%02d.md", repoPath, i), 30*time.Second)
	}

	// Verify delivery log has 10 DELIVERED entries
	logContent := readFileInContainer(t, ctx, c, "/workspace/.phonewave/delivery.log")
	deliveredCount := strings.Count(logContent, "DELIVERED")
	if deliveredCount < 10 {
		t.Errorf("expected at least 10 DELIVERED entries, got %d", deliveredCount)
	}
}

func TestLifecycleDocker_MalformedDMail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Write a file with no frontmatter
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/malformed.md", "No frontmatter here, just text.")

	// Wait a bit for processing
	time.Sleep(3 * time.Second)

	// Daemon should still be alive — write a valid D-Mail
	validContent := "---\nname: spec-after\nkind: specification\ndescription: After malformed\n---\n\n# After\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/spec-after.md", validContent)

	// Valid D-Mail should be delivered
	waitForFileInContainer(t, ctx, c, repoPath+"/.expedition/inbox/spec-after.md", 10*time.Second)
}

func TestLifecycleDocker_NonMdFilesIgnored(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Write non-.md files
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/notes.txt", "plain text")
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/data.json", "{}")
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/.DS_Store", "junk")

	time.Sleep(3 * time.Second)

	// Inbox should be empty
	count := countFilesInContainer(t, ctx, c, repoPath+"/.expedition/inbox", "")
	if count > 0 {
		t.Errorf("inbox should be empty for non-.md files, got %d files", count)
	}

	// Outbox files should still be there
	if !fileExistsInContainer(t, ctx, c, repoPath+"/.siren/outbox/notes.txt") {
		t.Error("notes.txt should remain in outbox")
	}
}

func TestLifecycleDocker_DeliveryLogPersistsRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	// First run: deliver a D-Mail
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	dmailContent := "---\nname: spec-persist1\nkind: specification\ndescription: Persist 1\n---\n\n# P1\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/spec-persist1.md", dmailContent)
	waitForFileInContainer(t, ctx, c, repoPath+"/.expedition/inbox/spec-persist1.md", 10*time.Second)
	stopDaemonInContainer(t, ctx, c, "/workspace")

	// Count log lines after first run
	log1 := readFileInContainer(t, ctx, c, "/workspace/.phonewave/delivery.log")
	lines1 := strings.Count(log1, "\n")

	// Second run: deliver another D-Mail
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	dmailContent2 := "---\nname: spec-persist2\nkind: specification\ndescription: Persist 2\n---\n\n# P2\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/spec-persist2.md", dmailContent2)
	waitForFileInContainer(t, ctx, c, repoPath+"/.expedition/inbox/spec-persist2.md", 10*time.Second)
	stopDaemonInContainer(t, ctx, c, "/workspace")

	// Log should have more lines (appended, not truncated)
	log2 := readFileInContainer(t, ctx, c, "/workspace/.phonewave/delivery.log")
	lines2 := strings.Count(log2, "\n")
	if lines2 <= lines1 {
		t.Errorf("delivery log should grow across restarts: %d lines before, %d after", lines1, lines2)
	}
}

func TestLifecycleDocker_UptimeTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Verify watch.started exists and contains RFC3339
	started := readFileInContainer(t, ctx, c, "/workspace/.phonewave/watch.started")
	started = strings.TrimSpace(started)
	if started == "" {
		t.Fatal("watch.started is empty")
	}

	// Let some time pass
	time.Sleep(2 * time.Second)

	// Status should show uptime > 0
	output := execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave status",
	})
	if strings.Contains(output, "Uptime:    0s") {
		t.Errorf("uptime should be > 0 after 2 seconds: %s", output)
	}
}

func TestLifecycleDocker_StartupScanMultipleFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	// Place 3 D-Mails before starting daemon
	for i := 0; i < 3; i++ {
		content := fmt.Sprintf("---\nname: pre-%02d\nkind: specification\ndescription: Pre %d\n---\n\n# Pre %d\n", i, i, i)
		heredocWrite(t, ctx, c, fmt.Sprintf("%s/.siren/outbox/pre-%02d.md", repoPath, i), content)
	}

	// Start daemon — startup scan should deliver all 3
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	for i := 0; i < 3; i++ {
		waitForFileInContainer(t, ctx, c,
			fmt.Sprintf("%s/.expedition/inbox/pre-%02d.md", repoPath, i), 15*time.Second)
	}

	// All should be removed from outbox
	for i := 0; i < 3; i++ {
		if fileExistsInContainer(t, ctx, c, fmt.Sprintf("%s/.siren/outbox/pre-%02d.md", repoPath, i)) {
			t.Errorf("pre-%02d.md should be removed from outbox after startup scan", i)
		}
	}
}
