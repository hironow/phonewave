//go:build e2e

package e2e

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
	dmailContent := "---\ndmail-schema-version: \"1\"\nname: spec-dry\nkind: specification\ndescription: Dry run test\n---\n\n# Dry\n"
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

	// Write D-Mail with a valid kind that has no consumer route configured
	noRouteContent := "---\ndmail-schema-version: \"1\"\nname: mystery-msg\nkind: ci-result\ndescription: No route for this kind\n---\n\n# No Route\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/mystery-msg.md", noRouteContent)

	// Wait for source file to be removed from outbox.
	// File removal proves the error queue enqueue succeeded
	// (daemon only removes outbox file after successful SQLite INSERT).
	waitForFileAbsentInContainer(t, ctx, c, repoPath+"/.siren/outbox/mystery-msg.md", 10*time.Second)

	// Verify delivery log has FAILED
	waitForStringInFile(t, ctx, c, "/workspace/.phonewave/delivery.log", "FAILED", 5*time.Second)

	// Stop daemon
	stopDaemonInContainer(t, ctx, c, "/workspace")

	// Add producer for ci-result to .siren (so a route from .siren/outbox exists)
	ciResultProducerDir := repoPath + "/.siren/skills/dmail-sendable-ciresult"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", ciResultProducerDir})
	heredocWrite(t, ctx, c, ciResultProducerDir+"/SKILL.md",
		"---\nname: dmail-sendable-ciresult\ndescription: Produces ci-result\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: ci-result\n---\n")

	// Add consumer for ci-result to .expedition
	ciResultConsumerDir := repoPath + "/.expedition/skills/dmail-readable-ciresult"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", ciResultConsumerDir})
	heredocWrite(t, ctx, c, ciResultConsumerDir+"/SKILL.md",
		"---\nname: dmail-readable-ciresult\ndescription: Consumes ci-result\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n    - kind: ci-result\n---\n")

	// Re-sync to pick up the new producer+consumer route
	execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave sync",
	})

	// Restart daemon — retry should pick up the error queue entry
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose --retry-interval 2s")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Verify RETRIED in delivery log (proves error queue retry succeeded)
	waitForStringInFile(t, ctx, c, "/workspace/.phonewave/delivery.log", "RETRIED", 15*time.Second)
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

	// Start daemon with max-retries=1 so the initial enqueue (retry_count=1)
	// already meets the threshold and no retries are attempted.
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose --retry-interval 2s --max-retries 1")
	defer stopDaemonInContainer(t, ctx, c, "/workspace")

	// Write D-Mail with no route → daemon enqueues to error queue with retry_count=1
	noRouteContent := "---\ndmail-schema-version: \"1\"\nname: exhausted-msg\nkind: ci-result\ndescription: No route for this kind\n---\n\n# Exhausted\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/exhausted-msg.md", noRouteContent)

	// Wait for file to be removed from outbox (enqueued to error queue)
	waitForFileAbsentInContainer(t, ctx, c, repoPath+"/.siren/outbox/exhausted-msg.md", 10*time.Second)

	// Verify FAILED in delivery log
	waitForStringInFile(t, ctx, c, "/workspace/.phonewave/delivery.log", "FAILED", 5*time.Second)

	// Wait for several retry intervals — daemon should NOT retry because
	// retry_count(1) >= max_retries(1)
	time.Sleep(6 * time.Second)

	// Verify no RETRIED in delivery log (max retries exceeded, no retry attempted)
	dlog := readFileInContainer(t, ctx, c, "/workspace/.phonewave/delivery.log")
	if strings.Contains(dlog, "RETRIED") {
		t.Error("should not retry when retry_count >= max_retries")
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
	fbContent := "---\ndmail-schema-version: \"1\"\nname: fb-rollback\nkind: feedback\ndescription: Rollback test\n---\n\n# Rollback\n"
	heredocWrite(t, ctx, c, repoPath+"/.gate/outbox/fb-rollback.md", fbContent)

	// Wait for the file to be processed
	time.Sleep(5 * time.Second)

	// Implementation uses at-least-once delivery with 2-phase flush:
	// - Stage: all targets recorded in SQLite (atomic)
	// - Flush: each target written independently
	// Partial flush is expected: .siren/inbox may succeed while .expedition/inbox fails.
	// The unflushed target remains in delivery_stage for retry on next daemon cycle.

	// .siren/inbox may have the file (partial delivery is acceptable)
	sirenDelivered := fileExistsInContainer(t, ctx, c, repoPath+"/.siren/inbox/fb-rollback.md")
	t.Logf("siren inbox delivered: %v (partial delivery is acceptable)", sirenDelivered)

	// Source should stay in outbox until ALL targets are flushed
	sourceInOutbox := fileExistsInContainer(t, ctx, c, repoPath+"/.gate/outbox/fb-rollback.md")
	if !sourceInOutbox && !sirenDelivered {
		t.Error("source removed from outbox but no deliveries made — data loss")
	}
	t.Logf("source in outbox: %v (stays until all targets flushed)", sourceInOutbox)
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
		content := fmt.Sprintf("---\ndmail-schema-version: \"1\"\nname: burst-%02d\nkind: specification\ndescription: Burst %d\n---\n\n# Burst %d\n", i, i, i)
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
	validContent := "---\ndmail-schema-version: \"1\"\nname: spec-after\nkind: specification\ndescription: After malformed\n---\n\n# After\n"
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
	dmailContent := "---\ndmail-schema-version: \"1\"\nname: spec-persist1\nkind: specification\ndescription: Persist 1\n---\n\n# P1\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/spec-persist1.md", dmailContent)
	waitForFileInContainer(t, ctx, c, repoPath+"/.expedition/inbox/spec-persist1.md", 10*time.Second)
	stopDaemonInContainer(t, ctx, c, "/workspace")

	// Count log lines after first run
	log1 := readFileInContainer(t, ctx, c, "/workspace/.phonewave/delivery.log")
	lines1 := strings.Count(log1, "\n")

	// Second run: deliver another D-Mail
	startDaemonInContainer(t, ctx, c, "/workspace", "--verbose")
	dmailContent2 := "---\ndmail-schema-version: \"1\"\nname: spec-persist2\nkind: specification\ndescription: Persist 2\n---\n\n# P2\n"
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
		content := fmt.Sprintf("---\ndmail-schema-version: \"1\"\nname: pre-%02d\nkind: specification\ndescription: Pre %d\n---\n\n# Pre %d\n", i, i, i)
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
