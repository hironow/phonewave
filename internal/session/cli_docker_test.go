//go:build docker

package session

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLifecycleDocker_MultiRepoInit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repo1 := "/workspace/repo1"
	repo2 := "/workspace/repo2"

	setupEcosystemInContainer(t, ctx, c, repo1)
	setupSecondRepoInContainer(t, ctx, c, repo2)

	// Init with two repo paths
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s %s", repo1, repo2),
	})

	if !fileExistsInContainer(t, ctx, c, "/workspace/phonewave.yaml") {
		t.Fatal("phonewave.yaml not created")
	}

	config := readFileInContainer(t, ctx, c, "/workspace/phonewave.yaml")
	if !strings.Contains(config, "repo1") {
		t.Error("config missing repo1")
	}
	if !strings.Contains(config, "repo2") {
		t.Error("config missing repo2")
	}

	// Verify state directory created
	if !dirExistsInContainer(t, ctx, c, "/workspace/.phonewave") {
		t.Error("state directory .phonewave not created")
	}
}

func TestLifecycleDocker_AddRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repo1 := "/workspace/repo1"
	repo2 := "/workspace/repo2"

	// Init with 1 repo
	setupEcosystemInContainer(t, ctx, c, repo1)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s", repo1),
	})

	configBefore := readFileInContainer(t, ctx, c, "/workspace/phonewave.yaml")

	// Add second repo
	setupSecondRepoInContainer(t, ctx, c, repo2)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave add %s", repo2),
	})

	configAfter := readFileInContainer(t, ctx, c, "/workspace/phonewave.yaml")
	if !strings.Contains(configAfter, "repo2") {
		t.Error("config missing repo2 after add")
	}
	if len(configAfter) <= len(configBefore) {
		t.Error("config did not grow after add")
	}
}

func TestLifecycleDocker_RemoveRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repo1 := "/workspace/repo1"
	repo2 := "/workspace/repo2"

	// Init with 2 repos
	setupEcosystemInContainer(t, ctx, c, repo1)
	setupSecondRepoInContainer(t, ctx, c, repo2)
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave init %s %s", repo1, repo2),
	})

	// Remove repo2
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("cd /workspace && phonewave remove %s", repo2),
	})

	config := readFileInContainer(t, ctx, c, "/workspace/phonewave.yaml")
	if strings.Contains(config, "repo2") {
		t.Error("config still contains repo2 after remove")
	}
	if !strings.Contains(config, "repo1") {
		t.Error("config should still contain repo1")
	}
}

func TestLifecycleDocker_Sync(t *testing.T) {
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

	// Add a new endpoint to existing repo
	newToolDir := repoPath + "/.oracle"
	for _, sub := range []string{"outbox", "inbox"} {
		execInContainer(t, ctx, c, []string{"mkdir", "-p", newToolDir + "/" + sub})
	}
	skillDir := newToolDir + "/skills/dmail-sendable"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", skillDir})
	heredocWrite(t, ctx, c, skillDir+"/SKILL.md", "---\nname: dmail-sendable\nproduces:\n  - kind: prophecy\n---\n")

	// Run sync
	output := execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave sync",
	})

	config := readFileInContainer(t, ctx, c, "/workspace/phonewave.yaml")
	if !strings.Contains(config, ".oracle") {
		t.Errorf("config missing .oracle after sync; output: %s", output)
	}
}

func TestLifecycleDocker_Doctor_Healthy(t *testing.T) {
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

	output := execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave doctor",
	})

	if !strings.Contains(output, "healthy") && !strings.Contains(output, "Healthy") {
		t.Errorf("doctor output does not indicate healthy: %s", output)
	}
}

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

func TestLifecycleDocker_Doctor_MissingDir(t *testing.T) {
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

	// Remove an endpoint directory
	execInContainer(t, ctx, c, []string{"rm", "-rf", repoPath + "/.siren"})

	exitCode, output := execInContainerNoFail(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave doctor",
	})

	if exitCode == 0 {
		t.Errorf("doctor should fail with missing directory; output: %s", output)
	}
}

func TestLifecycleDocker_StatusStopped(t *testing.T) {
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

	output := execInContainer(t, ctx, c, []string{
		"sh", "-c", "cd /workspace && phonewave status",
	})

	if !strings.Contains(output, "stopped") {
		t.Errorf("status should show 'stopped' when daemon is not running: %s", output)
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

func TestLifecycleDocker_ConfigFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	repoPath := "/workspace/repo"
	setupEcosystemInContainer(t, ctx, c, repoPath)

	customPath := "/workspace/custom/phonewave.yaml"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", "/workspace/custom"})
	execInContainer(t, ctx, c, []string{
		"sh", "-c", fmt.Sprintf("phonewave init --config %s %s", customPath, repoPath),
	})

	if !fileExistsInContainer(t, ctx, c, customPath) {
		t.Fatal("config not created at custom path")
	}

	// State dir should be at /workspace/custom/.phonewave/
	code, _ := execInContainerNoFail(t, ctx, c, []string{"test", "-d", "/workspace/custom/.phonewave"})
	if code != 0 {
		t.Error("state dir .phonewave not created alongside custom config")
	}
}

func TestLifecycleDocker_Version(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker test in short mode")
	}

	ctx := context.Background()
	c := buildTestContainer(t, ctx)

	output := execInContainer(t, ctx, c, []string{"phonewave", "version"})

	if !strings.Contains(output, "test") {
		t.Errorf("version output should contain 'test' from ldflags: %s", output)
	}
}
