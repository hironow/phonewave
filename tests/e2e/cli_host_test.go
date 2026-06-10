//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

// runPhonewaveInContainer runs the phonewave binary inside the container.
// Returns exitCode, stdout.
func runPhonewaveInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, workDir string, args ...string) (int, string) {
	t.Helper()
	cmd := []string{"sh", "-c", fmt.Sprintf("cd %s && phonewave %s", workDir, strings.Join(args, " "))}
	return execInContainerNoFail(t, ctx, c, cmd)
}

func TestCLI_MultiRepoInit(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_multi_init"
	repo1 := workDir + "/repo1"
	repo2 := workDir + "/repo2"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repo1})
	execInContainer(t, ctx, c, []string{"mkdir", "-p", repo2})

	setupEcosystemInContainer(t, ctx, c, repo1)
	setupSecondRepoInContainer(t, ctx, c, repo2)

	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "init", repo1, repo2)
	if code != 0 {
		t.Fatalf("init failed with code %d: %s", code, output)
	}

	config := readFileInContainer(t, ctx, c, workDir+"/.phonewave/config.yaml")
	if !strings.Contains(config, "repo1") {
		t.Error("config missing repo1")
	}
	if !strings.Contains(config, "repo2") {
		t.Error("config missing repo2")
	}

	if !dirExistsInContainer(t, ctx, c, workDir+"/.phonewave") {
		t.Error("state directory .phonewave not created")
	}
}

func TestCLI_AddRepo(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_add_repo"
	repo1 := workDir + "/repo1"
	repo2 := workDir + "/repo2"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repo1})
	execInContainer(t, ctx, c, []string{"mkdir", "-p", repo2})

	setupEcosystemInContainer(t, ctx, c, repo1)
	runPhonewaveInContainer(t, ctx, c, workDir, "init", repo1)

	configBefore := readFileInContainer(t, ctx, c, workDir+"/.phonewave/config.yaml")

	setupSecondRepoInContainer(t, ctx, c, repo2)
	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "add", repo2)
	if code != 0 {
		t.Fatalf("add failed with code %d: %s", code, output)
	}

	configAfter := readFileInContainer(t, ctx, c, workDir+"/.phonewave/config.yaml")
	if !strings.Contains(configAfter, "repo2") {
		t.Error("config missing repo2 after add")
	}
	if len(configAfter) <= len(configBefore) {
		t.Error("config did not grow after add")
	}
}

func TestCLI_RemoveRepo(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_remove_repo"
	repo1 := workDir + "/repo1"
	repo2 := workDir + "/repo2"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repo1})
	execInContainer(t, ctx, c, []string{"mkdir", "-p", repo2})

	setupEcosystemInContainer(t, ctx, c, repo1)
	setupSecondRepoInContainer(t, ctx, c, repo2)
	runPhonewaveInContainer(t, ctx, c, workDir, "init", repo1, repo2)

	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "remove", repo2)
	if code != 0 {
		t.Fatalf("remove failed with code %d: %s", code, output)
	}

	config := readFileInContainer(t, ctx, c, workDir+"/.phonewave/config.yaml")
	if strings.Contains(config, "repo2") {
		t.Error("config still contains repo2 after remove")
	}
	if !strings.Contains(config, "repo1") {
		t.Error("config should still contain repo1")
	}
}

func TestCLI_Sync(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_sync"
	repoPath := workDir + "/repo"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repoPath})
	setupEcosystemInContainer(t, ctx, c, repoPath)
	runPhonewaveInContainer(t, ctx, c, workDir, "init", repoPath)

	// Add a new endpoint
	oracleDir := repoPath + "/.oracle"
	for _, sub := range []string{"outbox", "inbox"} {
		execInContainer(t, ctx, c, []string{"mkdir", "-p", oracleDir + "/" + sub})
	}
	skillDir := oracleDir + "/skills/dmail-sendable"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", skillDir})
	heredocWrite(t, ctx, c, skillDir+"/SKILL.md", "---\nname: dmail-sendable\ndescription: Oracle predictions\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: ci-result\n---\n")

	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "sync")
	if code != 0 {
		t.Fatalf("sync failed with code %d: %s", code, output)
	}

	config := readFileInContainer(t, ctx, c, workDir+"/.phonewave/config.yaml")
	if !strings.Contains(config, ".oracle") {
		t.Error("config missing .oracle after sync")
	}
}

func TestCLI_Doctor_Healthy(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_doctor_healthy"
	repoPath := workDir + "/repo"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repoPath})
	setupEcosystemInContainer(t, ctx, c, repoPath)
	runPhonewaveInContainer(t, ctx, c, workDir, "init", repoPath)

	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "doctor")
	if code != 0 {
		t.Fatalf("doctor failed with code %d: %s", code, output)
	}

	if !strings.Contains(output, "All checks passed") {
		t.Errorf("doctor output does not indicate healthy: %s", output)
	}
}

func TestCLI_Doctor_MissingDir(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_doctor_missing"
	repoPath := workDir + "/repo"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repoPath})
	setupEcosystemInContainer(t, ctx, c, repoPath)
	runPhonewaveInContainer(t, ctx, c, workDir, "init", repoPath)

	// Remove an endpoint directory inside container
	execInContainer(t, ctx, c, []string{"rm", "-rf", repoPath + "/.siren"})

	code, _ := runPhonewaveInContainer(t, ctx, c, workDir, "doctor")
	if code == 0 {
		t.Error("doctor should fail with missing directory")
	}
}

func TestCLI_StatusStopped(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_status_stopped"
	repoPath := workDir + "/repo"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repoPath})
	setupEcosystemInContainer(t, ctx, c, repoPath)
	runPhonewaveInContainer(t, ctx, c, workDir, "init", repoPath)

	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "status")
	if code != 0 {
		t.Fatalf("status failed with code %d: %s", code, output)
	}

	if !strings.Contains(output, "stopped") {
		t.Errorf("status should show 'stopped' when daemon is not running: %s", output)
	}
}

func TestCLI_ConfigFlag(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_config_flag"
	repoPath := workDir + "/repo"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", repoPath})
	setupEcosystemInContainer(t, ctx, c, repoPath)

	customStateDir := workDir + "/custom/.phonewave"
	execInContainer(t, ctx, c, []string{"mkdir", "-p", customStateDir})
	customPath := customStateDir + "/config.yaml"

	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "init", "--config", customPath, repoPath)
	if code != 0 {
		t.Fatalf("init with --config failed with code %d: %s", code, output)
	}

	if !fileExistsInContainer(t, ctx, c, customPath) {
		t.Fatal("config not created at custom path")
	}

	if !dirExistsInContainer(t, ctx, c, customStateDir) {
		t.Error("state dir .phonewave not created at custom location")
	}
}

func TestCLI_Version(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_version"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", workDir})
	code, output := runPhonewaveInContainer(t, ctx, c, workDir, "version")
	if code != 0 {
		t.Fatalf("version failed with code %d: %s", code, output)
	}

	if !strings.Contains(output, "phonewave") {
		t.Errorf("version output should contain 'phonewave': %s", output)
	}
}

func TestCLI_MCPServerToolsList(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	workDir := "/workspace/t_mcp"

	execInContainer(t, ctx, c, []string{"mkdir", "-p", workDir})

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	cmd := []string{"sh", "-c", fmt.Sprintf("echo '%s' | phonewave mcp", input)}
	code, stdout := execInContainerNoFail(t, ctx, c, cmd)
	if code != 0 {
		t.Fatalf("mcp command failed with code %d: %s", code, stdout)
	}

	idx := strings.Index(stdout, `{"jsonrpc"`)
	if idx < 0 {
		t.Fatalf("no JSON-RPC response found in stdout: %s", stdout)
	}
	jsonStr := stdout[idx:]
	lastBrace := strings.LastIndex(jsonStr, "}")
	if lastBrace >= 0 {
		jsonStr = jsonStr[:lastBrace+1]
	}

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON-RPC response: %v\nraw: %s", err, jsonStr)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	if resp.ID != 1 {
		t.Errorf("expected id 1, got %d", resp.ID)
	}

	expectedTools := map[string]bool{
		"ping":          false,
		"outbox_status": false,
		"inbox_status":  false,
	}

	for _, tool := range resp.Result.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("missing expected tool in MCP response: %s", name)
		}
	}
}
