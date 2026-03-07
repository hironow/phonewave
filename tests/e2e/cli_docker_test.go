//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Docker-dependent CLI tests: these require a running daemon inside the container.

func TestLifecycleDocker_Doctor_WhileRunning(t *testing.T) {
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

	output := execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave doctor",
	})

	if !strings.Contains(output, "PID") && !strings.Contains(output, "running") {
		t.Errorf("doctor output should mention running daemon: %s", output)
	}
}

func TestLifecycleDocker_StatusRunning(t *testing.T) {
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

	// Deliver something so stats have data
	dmailContent := "---\ndmail-schema-version: \"1\"\nname: spec-status\nkind: specification\ndescription: Status test\n---\n\n# Status\n"
	heredocWrite(t, ctx, c, repoPath+"/.siren/outbox/spec-status.md", dmailContent)
	waitForFileInContainer(t, ctx, c, repoPath+"/.expedition/inbox/spec-status.md", 10*time.Second)

	output := execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave status",
	})

	if !strings.Contains(output, "running") {
		t.Errorf("status should show 'running': %s", output)
	}
	if !strings.Contains(output, "PID") {
		t.Errorf("status should show PID: %s", output)
	}
}
