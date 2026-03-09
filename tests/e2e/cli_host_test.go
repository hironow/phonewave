//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runPhonewave runs the phonewave binary with given args in the specified workdir.
// Returns stdout, stderr, and error.
func runPhonewave(t *testing.T, workDir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(phonewaveBin(), args...)
	cmd.Dir = workDir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Logf("phonewave %v failed: err=%v\nstdout: %s\nstderr: %s", args, err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String(), err
}

// setupEcosystemOnHost creates the 3-tool ecosystem in a tempdir.
func setupEcosystemOnHost(t *testing.T, repoPath string) {
	t.Helper()

	type toolDef struct {
		dir      string
		produces string
		consumes []string
	}

	tools := []toolDef{
		{".siren", "specification", []string{"design-feedback"}},
		{".expedition", "report", []string{"specification", "design-feedback"}},
		{".gate", "design-feedback", []string{"report"}},
	}

	for _, tool := range tools {
		for _, sub := range []string{"outbox", "inbox"} {
			os.MkdirAll(filepath.Join(repoPath, tool.dir, sub), 0o755)
		}

		// dmail-sendable SKILL.md
		sendableDir := filepath.Join(repoPath, tool.dir, "skills", "dmail-sendable")
		os.MkdirAll(sendableDir, 0o755)
		content := fmt.Sprintf("---\nname: dmail-sendable\ndescription: Produces D-Mail messages\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: %s\n---\n", tool.produces)
		os.WriteFile(filepath.Join(sendableDir, "SKILL.md"), []byte(content), 0o644)

		// dmail-readable SKILL.md
		readableDir := filepath.Join(repoPath, tool.dir, "skills", "dmail-readable")
		os.MkdirAll(readableDir, 0o755)
		var consumesYAML string
		for i, k := range tool.consumes {
			if i == 0 {
				consumesYAML += fmt.Sprintf("    - kind: %s\n", k)
			} else {
				consumesYAML += fmt.Sprintf("    - kind: %s\n", k)
			}
		}
		readableContent := fmt.Sprintf("---\nname: dmail-readable\ndescription: Consumes D-Mail messages\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n%s---\n", consumesYAML)
		os.WriteFile(filepath.Join(readableDir, "SKILL.md"), []byte(readableContent), 0o644)
	}
}

// setupSecondRepoOnHost creates a second repo with .beacon and .monitor endpoints.
func setupSecondRepoOnHost(t *testing.T, repoPath string) {
	t.Helper()
	tools := []struct {
		dir, produces, consumes string
	}{
		{".beacon", "convergence", ""},
		{".monitor", "", "convergence"},
	}
	for _, tool := range tools {
		for _, sub := range []string{"outbox", "inbox"} {
			os.MkdirAll(filepath.Join(repoPath, tool.dir, sub), 0o755)
		}
		if tool.produces != "" {
			skillDir := filepath.Join(repoPath, tool.dir, "skills", "dmail-sendable")
			os.MkdirAll(skillDir, 0o755)
			content := fmt.Sprintf("---\nname: dmail-sendable\ndescription: Produces D-Mail messages\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: %s\n---\n", tool.produces)
			os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)
		}
		if tool.consumes != "" {
			skillDir := filepath.Join(repoPath, tool.dir, "skills", "dmail-readable")
			os.MkdirAll(skillDir, 0o755)
			content := fmt.Sprintf("---\nname: dmail-readable\ndescription: Consumes D-Mail messages\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n    - kind: %s\n---\n", tool.consumes)
			os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)
		}
	}
}

func TestCLI_MultiRepoInit(t *testing.T) {
	workDir := t.TempDir()
	repo1 := filepath.Join(workDir, "repo1")
	repo2 := filepath.Join(workDir, "repo2")
	os.MkdirAll(repo1, 0o755)
	os.MkdirAll(repo2, 0o755)

	setupEcosystemOnHost(t, repo1)
	setupSecondRepoOnHost(t, repo2)

	_, _, err := runPhonewave(t, workDir, "init", repo1, repo2)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	config, err := os.ReadFile(filepath.Join(workDir, ".phonewave", "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(config), "repo1") {
		t.Error("config missing repo1")
	}
	if !strings.Contains(string(config), "repo2") {
		t.Error("config missing repo2")
	}

	if _, err := os.Stat(filepath.Join(workDir, ".phonewave")); os.IsNotExist(err) {
		t.Error("state directory .phonewave not created")
	}
}

func TestCLI_AddRepo(t *testing.T) {
	workDir := t.TempDir()
	repo1 := filepath.Join(workDir, "repo1")
	repo2 := filepath.Join(workDir, "repo2")
	os.MkdirAll(repo1, 0o755)
	os.MkdirAll(repo2, 0o755)

	setupEcosystemOnHost(t, repo1)
	runPhonewave(t, workDir, "init", repo1)

	configBefore, _ := os.ReadFile(filepath.Join(workDir, ".phonewave", "config.yaml"))

	setupSecondRepoOnHost(t, repo2)
	_, _, err := runPhonewave(t, workDir, "add", repo2)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	configAfter, _ := os.ReadFile(filepath.Join(workDir, ".phonewave", "config.yaml"))
	if !strings.Contains(string(configAfter), "repo2") {
		t.Error("config missing repo2 after add")
	}
	if len(configAfter) <= len(configBefore) {
		t.Error("config did not grow after add")
	}
}

func TestCLI_RemoveRepo(t *testing.T) {
	workDir := t.TempDir()
	repo1 := filepath.Join(workDir, "repo1")
	repo2 := filepath.Join(workDir, "repo2")
	os.MkdirAll(repo1, 0o755)
	os.MkdirAll(repo2, 0o755)

	setupEcosystemOnHost(t, repo1)
	setupSecondRepoOnHost(t, repo2)
	runPhonewave(t, workDir, "init", repo1, repo2)

	_, _, err := runPhonewave(t, workDir, "remove", repo2)
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	config, _ := os.ReadFile(filepath.Join(workDir, ".phonewave", "config.yaml"))
	if strings.Contains(string(config), "repo2") {
		t.Error("config still contains repo2 after remove")
	}
	if !strings.Contains(string(config), "repo1") {
		t.Error("config should still contain repo1")
	}
}

func TestCLI_Sync(t *testing.T) {
	workDir := t.TempDir()
	repoPath := filepath.Join(workDir, "repo")
	os.MkdirAll(repoPath, 0o755)

	setupEcosystemOnHost(t, repoPath)
	runPhonewave(t, workDir, "init", repoPath)

	// Add a new endpoint
	oracleDir := filepath.Join(repoPath, ".oracle")
	for _, sub := range []string{"outbox", "inbox"} {
		os.MkdirAll(filepath.Join(oracleDir, sub), 0o755)
	}
	skillDir := filepath.Join(oracleDir, "skills", "dmail-sendable")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: dmail-sendable\ndescription: Oracle predictions\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: ci-result\n---\n"), 0o644)

	_, _, err := runPhonewave(t, workDir, "sync")
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	config, _ := os.ReadFile(filepath.Join(workDir, ".phonewave", "config.yaml"))
	if !strings.Contains(string(config), ".oracle") {
		t.Error("config missing .oracle after sync")
	}
}

func TestCLI_Doctor_Healthy(t *testing.T) {
	workDir := t.TempDir()
	repoPath := filepath.Join(workDir, "repo")
	os.MkdirAll(repoPath, 0o755)

	setupEcosystemOnHost(t, repoPath)
	runPhonewave(t, workDir, "init", repoPath)

	stdout, stderr, err := runPhonewave(t, workDir, "doctor")
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}

	combined := stdout + stderr
	if !strings.Contains(strings.ToLower(combined), "healthy") {
		t.Errorf("doctor output does not indicate healthy: %s", combined)
	}
}

func TestCLI_Doctor_MissingDir(t *testing.T) {
	workDir := t.TempDir()
	repoPath := filepath.Join(workDir, "repo")
	os.MkdirAll(repoPath, 0o755)

	setupEcosystemOnHost(t, repoPath)
	runPhonewave(t, workDir, "init", repoPath)

	// Remove an endpoint directory
	os.RemoveAll(filepath.Join(repoPath, ".siren"))

	_, _, err := runPhonewave(t, workDir, "doctor")
	if err == nil {
		t.Error("doctor should fail with missing directory")
	}
}

func TestCLI_StatusStopped(t *testing.T) {
	workDir := t.TempDir()
	repoPath := filepath.Join(workDir, "repo")
	os.MkdirAll(repoPath, 0o755)

	setupEcosystemOnHost(t, repoPath)
	runPhonewave(t, workDir, "init", repoPath)

	stdout, _, err := runPhonewave(t, workDir, "status")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if !strings.Contains(stdout, "stopped") {
		t.Errorf("status should show 'stopped' when daemon is not running: %s", stdout)
	}
}

func TestCLI_ConfigFlag(t *testing.T) {
	workDir := t.TempDir()
	repoPath := filepath.Join(workDir, "repo")
	os.MkdirAll(repoPath, 0o755)

	setupEcosystemOnHost(t, repoPath)

	customStateDir := filepath.Join(workDir, "custom", ".phonewave")
	os.MkdirAll(customStateDir, 0o755)
	customPath := filepath.Join(customStateDir, "config.yaml")

	_, _, err := runPhonewave(t, workDir, "init", "--config", customPath, repoPath)
	if err != nil {
		t.Fatalf("init with --config failed: %v", err)
	}

	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Fatal("config not created at custom path")
	}

	if _, err := os.Stat(customStateDir); os.IsNotExist(err) {
		t.Error("state dir .phonewave not created at custom location")
	}
}

func TestCLI_Version(t *testing.T) {
	stdout, _, err := runPhonewave(t, t.TempDir(), "version")
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}

	if !strings.Contains(stdout, "phonewave") {
		t.Errorf("version output should contain 'phonewave': %s", stdout)
	}
}
