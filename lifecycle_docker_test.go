//go:build docker

package phonewave

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// execInContainer runs a command and returns stdout. Fails the test on non-zero exit.
func execInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) string {
	t.Helper()
	exitCode, reader, err := c.Exec(ctx, cmd)
	if err != nil {
		t.Fatalf("exec %v: %v", cmd, err)
	}
	output, _ := io.ReadAll(reader)
	if exitCode != 0 {
		t.Fatalf("exec %v exited %d: %s", cmd, exitCode, string(output))
	}
	return string(output)
}

// execInContainerNoFail runs a command and returns exit code + output without failing.
func execInContainerNoFail(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) (int, string) {
	t.Helper()
	exitCode, reader, err := c.Exec(ctx, cmd)
	if err != nil {
		t.Fatalf("exec %v: %v", cmd, err)
	}
	output, _ := io.ReadAll(reader)
	return exitCode, string(output)
}

// fileExistsInContainer checks if a file exists inside the container.
func fileExistsInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string) bool {
	t.Helper()
	code, _ := execInContainerNoFail(t, ctx, c, []string{"test", "-f", path})
	return code == 0
}

// waitForFileInContainer polls until a file exists inside the container.
func waitForFileInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for file in container: %s", path)
		default:
			if fileExistsInContainer(t, ctx, c, path) {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// waitForFileAbsentInContainer polls until a file is gone inside the container.
func waitForFileAbsentInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for file removal in container: %s", path)
		default:
			if !fileExistsInContainer(t, ctx, c, path) {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// heredocWrite writes content to a file inside the container using sh heredoc.
// This ensures real write(2) syscalls that trigger inotify/fsnotify.
func heredocWrite(t *testing.T, ctx context.Context, c testcontainers.Container, path, content string) {
	t.Helper()
	// Use printf to avoid heredoc delimiter issues with YAML
	cmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p \"$(dirname '%s')\" && printf '%%s' '%s' > '%s'",
		path, strings.ReplaceAll(content, "'", "'\\''"), path)}
	execInContainer(t, ctx, c, cmd)
}

// setupEcosystemInContainer creates the 3-tool ecosystem inside the container.
func setupEcosystemInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, repoPath string) {
	t.Helper()

	type toolDef struct {
		dir      string
		produces string
		consumes string
	}

	tools := []toolDef{
		{".siren", "specification", "feedback"},
		{".expedition", "report", "specification\n  - kind: feedback"},
		{".gate", "feedback", "report"},
	}

	for _, tool := range tools {
		// Create directory structure
		for _, sub := range []string{"outbox", "inbox"} {
			execInContainer(t, ctx, c, []string{
				"mkdir", "-p", fmt.Sprintf("%s/%s/%s", repoPath, tool.dir, sub),
			})
		}

		// dmail-sendable SKILL.md
		sendableDir := fmt.Sprintf("%s/%s/skills/dmail-sendable", repoPath, tool.dir)
		execInContainer(t, ctx, c, []string{"mkdir", "-p", sendableDir})
		sendableContent := fmt.Sprintf("---\nname: dmail-sendable\nproduces:\n  - kind: %s\n---\n", tool.produces)
		heredocWrite(t, ctx, c, sendableDir+"/SKILL.md", sendableContent)

		// dmail-readable SKILL.md
		readableDir := fmt.Sprintf("%s/%s/skills/dmail-readable", repoPath, tool.dir)
		execInContainer(t, ctx, c, []string{"mkdir", "-p", readableDir})
		readableContent := fmt.Sprintf("---\nname: dmail-readable\nconsumes:\n  - kind: %s\n---\n", tool.consumes)
		heredocWrite(t, ctx, c, readableDir+"/SKILL.md", readableContent)
	}
}

func TestLifecycleDocker_SingleContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	repoPath := "/workspace/repo"

	// =====================================================================
	// Phase 1: Build and start container
	// =====================================================================
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "testdata/Dockerfile.test",
		},
		WaitingFor: wait.ForExec([]string{"phonewave", "--version"}).
			WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("terminate container: %v", err)
		}
	}()

	// Verify phonewave binary works
	versionOutput := execInContainer(t, ctx, container, []string{"phonewave", "--version"})
	if !strings.Contains(versionOutput, "phonewave") {
		t.Fatalf("unexpected version output: %s", versionOutput)
	}

	// =====================================================================
	// Phase 2: Setup ecosystem inside container
	// =====================================================================
	setupEcosystemInContainer(t, ctx, container, repoPath)

	// =====================================================================
	// Phase 3: Run phonewave init
	// =====================================================================
	execInContainer(t, ctx, container, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repoPath),
	})

	if !fileExistsInContainer(t, ctx, container, "/workspace/phonewave.yaml") {
		t.Fatal("phonewave.yaml not created after init")
	}

	// =====================================================================
	// Phase 4: Place pre-existing D-Mail, then start daemon
	// =====================================================================
	dmailContent := "---\nname: spec-docker\nkind: specification\ndescription: Docker test\n---\n\n# Docker Spec\n"
	heredocWrite(t, ctx, container, repoPath+"/.siren/outbox/spec-docker.md", dmailContent)

	// Start daemon in background
	execInContainer(t, ctx, container, []string{
		"sh", "-c", "cd /workspace && nohup phonewave run --verbose > /tmp/phonewave.log 2>&1 &",
	})

	// Wait for PID file (daemon started)
	waitForFileInContainer(t, ctx, container, "/workspace/.phonewave/watch.pid", 15*time.Second)

	// =====================================================================
	// Phase 5: Verify startup scan delivered pre-existing file
	// =====================================================================
	waitForFileInContainer(t, ctx, container,
		repoPath+"/.expedition/inbox/spec-docker.md", 10*time.Second)
	waitForFileAbsentInContainer(t, ctx, container,
		repoPath+"/.siren/outbox/spec-docker.md", 10*time.Second)

	// =====================================================================
	// Phase 6: Runtime delivery — write new D-Mail via exec
	// =====================================================================
	runtimeContent := "---\nname: spec-runtime\nkind: specification\ndescription: Runtime test\n---\n\n# Runtime\n"
	heredocWrite(t, ctx, container, repoPath+"/.siren/outbox/spec-runtime.md", runtimeContent)

	waitForFileInContainer(t, ctx, container,
		repoPath+"/.expedition/inbox/spec-runtime.md", 10*time.Second)

	// =====================================================================
	// Phase 7: Multi-target delivery — feedback → siren + expedition
	// =====================================================================
	feedbackContent := "---\nname: fb-docker\nkind: feedback\ndescription: Docker feedback\n---\n\n# Feedback\n"
	heredocWrite(t, ctx, container, repoPath+"/.gate/outbox/fb-docker.md", feedbackContent)

	waitForFileInContainer(t, ctx, container,
		repoPath+"/.siren/inbox/fb-docker.md", 10*time.Second)
	waitForFileInContainer(t, ctx, container,
		repoPath+"/.expedition/inbox/fb-docker.md", 10*time.Second)

	// =====================================================================
	// Phase 8: Verify delivery log
	// =====================================================================
	_, logOutput := execInContainerNoFail(t, ctx, container,
		[]string{"cat", "/workspace/.phonewave/delivery.log"})

	if !strings.Contains(logOutput, "DELIVERED") {
		t.Error("delivery log missing DELIVERED entries")
	}
	if !strings.Contains(logOutput, "kind=specification") {
		t.Error("delivery log missing kind=specification")
	}
	if !strings.Contains(logOutput, "kind=feedback") {
		t.Error("delivery log missing kind=feedback")
	}

	// =====================================================================
	// Phase 9: Stop daemon and verify cleanup
	// =====================================================================
	// Kill daemon using PID from file — keep it all inside the container
	// to avoid Docker stream multiplexing artifacts in PID output.
	execInContainer(t, ctx, container, []string{
		"sh", "-c", "kill $(cat /workspace/.phonewave/watch.pid)",
	})

	// Wait for PID file removal (graceful shutdown)
	waitForFileAbsentInContainer(t, ctx, container,
		"/workspace/.phonewave/watch.pid", 10*time.Second)

	// Verify daemon log shows shutdown message
	_, daemonLog := execInContainerNoFail(t, ctx, container, []string{"cat", "/tmp/phonewave.log"})
	if !strings.Contains(daemonLog, "Daemon stopped") && !strings.Contains(daemonLog, "Shutting down") {
		t.Logf("daemon log: %s", daemonLog)
	}
}
