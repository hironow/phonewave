package session_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestDoctor_RepairReceivesStateDir(t *testing.T) {
	// given — repairSyncFn now receives stateDir (not configPath)
	baseDir := t.TempDir()
	stateDir := filepath.Join(baseDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Do NOT create .run/resolved.yaml so repair triggers

	cfg := &domain.Config{}

	// Track which stateDir repairSyncFn receives
	var receivedStateDir string
	cleanup := session.OverrideRepairSync(func(c *domain.Config, sd string) error {
		receivedStateDir = sd
		// Create resolved.yaml to satisfy the check
		runDir := filepath.Join(sd, ".run")
		os.MkdirAll(runDir, 0755)
		return os.WriteFile(
			filepath.Join(runDir, domain.ResolvedStateFile),
			[]byte("routes: []\n"), 0644)
	})
	defer cleanup()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true, "")

	// then — repairSyncFn should receive stateDir
	if receivedStateDir != stateDir {
		t.Errorf("repairSyncFn received %q, want stateDir %q", receivedStateDir, stateDir)
	}
	// report should show fixed
	hasFixed := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckFixed && strings.Contains(issue.Message, "resolved") {
			hasFixed = true
		}
	}
	if !hasFixed {
		t.Errorf("expected fixed issue for resolved.yaml, got: %v", report.Checks)
	}
}

func TestDoctor_RepairStalePID(t *testing.T) {
	// given — stale PID file with non-existent process
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	pidPath := filepath.Join(stateDir, "watch.pid")
	os.WriteFile(pidPath, []byte("999999999"), 0644)

	cfg := &domain.Config{}

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true, "")

	// then — stale PID file should be removed
	if _, err := os.Stat(pidPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected stale PID file to be removed")
	}
	// daemon status should reflect cleanup
	if report.DaemonStatus.Running {
		t.Error("daemon should not be running after stale PID cleanup")
	}
}

func TestDoctor_RepairSkillsRef_UvAvailable_NoSubmodule(t *testing.T) {
	// given — uv is on PATH but skills-ref is not, and no submodule
	// After install, skills-ref becomes available on PATH
	cfg := &domain.Config{}
	stateDir := filepath.Join(t.TempDir(), domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	var installCalled bool
	cleanup := session.OverrideRepairInstallSkillsRef(func() error {
		installCalled = true
		return nil
	})
	defer cleanup()

	// skills-ref is NOT found initially (first call), but IS found after install (second call)
	skillsRefCallCount := 0
	cleanup2 := session.OverrideLookPath(func(name string) (string, error) {
		if name == "uv" {
			return "/usr/bin/uv", nil
		}
		if name == "skills-ref" {
			skillsRefCallCount++
			if skillsRefCallCount > 1 {
				// After install, skills-ref is on PATH
				return "/usr/local/bin/skills-ref", nil
			}
		}
		return "", exec.ErrNotFound
	})
	defer cleanup2()

	cleanup3 := session.OverrideFindSkillsRefDir(func() string {
		return "" // no submodule
	})
	defer cleanup3()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true, "")

	// then — install should have been called
	if !installCalled {
		t.Error("expected uv tool install skills-ref to be called")
	}
	hasFixed := false
	for _, issue := range report.Checks {
		if issue.Name == "skills-ref" && issue.Status == domain.CheckFixed {
			hasFixed = true
		}
	}
	if !hasFixed {
		t.Errorf("expected fixed issue for skills-ref, got: %v", report.Checks)
	}
}

func TestDoctor_RepairSkillsRef_SubmoduleAvailable_NoInstall(t *testing.T) {
	// given — uv on PATH, skills-ref NOT on PATH, but submodule IS available
	cfg := &domain.Config{}
	stateDir := filepath.Join(t.TempDir(), domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	var installCalled bool
	cleanup := session.OverrideRepairInstallSkillsRef(func() error {
		installCalled = true
		return nil
	})
	defer cleanup()

	cleanup2 := session.OverrideLookPath(func(name string) (string, error) {
		if name == "uv" {
			return "/usr/bin/uv", nil
		}
		return "", exec.ErrNotFound
	})
	defer cleanup2()

	cleanup3 := session.OverrideFindSkillsRefDir(func() string {
		return "/some/submodule/path" // submodule available
	})
	defer cleanup3()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true, "")

	// then — install should NOT have been called (submodule suffices)
	if installCalled {
		t.Error("should not install skills-ref when submodule is available")
	}
	hasOK := false
	for _, issue := range report.Checks {
		if issue.Name == "skills-ref" && issue.Status == domain.CheckOK {
			hasOK = true
		}
	}
	if !hasOK {
		t.Errorf("expected OK issue for skills-ref with submodule, got: %v", report.Checks)
	}
}

func TestDoctor_RepairResolvedState(t *testing.T) {
	// given — resolved.yaml does not exist
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	runDir := filepath.Join(stateDir, ".run")
	os.MkdirAll(runDir, 0755)
	os.MkdirAll(filepath.Join(repoDir, ".siren", "outbox"), 0755)
	os.MkdirAll(filepath.Join(repoDir, ".siren", "inbox"), 0755)

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"spec"}},
				},
			},
		},
	}

	// Inject repair resolved-state regeneration function
	var repairCalled bool
	cleanup := session.OverrideRepairSync(func(c *domain.Config, sd string) error {
		repairCalled = true
		runDir2 := filepath.Join(sd, ".run")
		os.MkdirAll(runDir2, 0755)
		return os.WriteFile(
			filepath.Join(runDir2, domain.ResolvedStateFile),
			[]byte("routes: []\n"), 0644)
	})
	defer cleanup()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true, "")

	// then
	if !repairCalled {
		t.Error("expected repair sync to be called for missing resolved.yaml")
	}
	hasFixed := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckFixed && strings.Contains(issue.Message, "resolved") {
			hasFixed = true
		}
	}
	if !hasFixed {
		t.Errorf("expected fixed issue for resolved.yaml, got: %v", report.Checks)
	}
}

func TestDoctor_HealthyEcosystem(t *testing.T) {
	// given — a fully set up repo with all dirs and SKILL.md files
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)

	for _, dir := range []string{
		filepath.Join(repoDir, ".siren", "outbox"),
		filepath.Join(repoDir, ".siren", "inbox"),
		filepath.Join(repoDir, ".siren", "skills", "dmail-sendable"),
		filepath.Join(repoDir, ".siren", "skills", "dmail-readable"),
		filepath.Join(repoDir, ".expedition", "outbox"),
		filepath.Join(repoDir, ".expedition", "inbox"),
		filepath.Join(repoDir, ".expedition", "skills", "dmail-sendable"),
		filepath.Join(repoDir, ".expedition", "skills", "dmail-readable"),
		stateDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Write SKILL.md files (metadata-nested format, schema v1)
	writeSkillFile(t, filepath.Join(repoDir, ".siren", "skills", "dmail-sendable", "SKILL.md"),
		"---\nname: dmail-sendable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: specification\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".siren", "skills", "dmail-readable", "SKILL.md"),
		"---\nname: dmail-readable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n    - kind: design-feedback\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".expedition", "skills", "dmail-sendable", "SKILL.md"),
		"---\nname: dmail-sendable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: report\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".expedition", "skills", "dmail-readable", "SKILL.md"),
		"---\nname: dmail-readable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n    - kind: specification\n---\n")

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
				},
			},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then
	if !report.Healthy {
		t.Errorf("expected healthy ecosystem, got issues: %v", report.Checks)
	}
	if len(report.Endpoints) != 2 {
		t.Errorf("endpoints = %d, want 2", len(report.Endpoints))
	}
}

func TestDoctor_MissingDirs(t *testing.T) {
	// given — repo path exists but outbox/inbox dirs are missing
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create endpoint dir but NOT outbox/inbox
	if err := os.MkdirAll(filepath.Join(repoDir, ".siren"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should have warnings about missing dirs but auto-create them
	hasCreated := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckFixed {
			hasCreated = true
		}
	}
	if !hasCreated {
		t.Error("expected 'fixed' issues for auto-created directories")
	}

	// outbox and inbox should now exist
	if _, err := os.Stat(filepath.Join(repoDir, ".siren", "outbox")); errors.Is(err, fs.ErrNotExist) {
		t.Error("outbox should have been auto-created")
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".siren", "inbox")); errors.Is(err, fs.ErrNotExist) {
		t.Error("inbox should have been auto-created")
	}
}

func TestDoctor_MissingRepoPath(t *testing.T) {
	// given — config references a non-existent repository path
	stateDir := t.TempDir()

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: "/nonexistent/repo/path",
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then
	if report.Healthy {
		t.Error("expected unhealthy with missing repo path")
	}
	hasError := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckFail {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error-level issue for missing repo path")
	}
}

func TestDoctor_InvalidKindInSkillMD(t *testing.T) {
	// given — SKILL.md with an invalid kind
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)

	for _, dir := range []string{
		filepath.Join(repoDir, ".siren", "outbox"),
		filepath.Join(repoDir, ".siren", "inbox"),
		filepath.Join(repoDir, ".siren", "skills", "dmail-sendable"),
		stateDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	writeSkillFile(t, filepath.Join(repoDir, ".siren", "skills", "dmail-sendable", "SKILL.md"),
		"---\nname: dmail-sendable\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: invalid_type\n---\n")

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should have a warning about invalid kind
	hasKindWarn := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckWarn && strings.Contains(issue.Message, "invalid D-Mail kind") {
			hasKindWarn = true
		}
	}
	if !hasKindWarn {
		t.Errorf("expected warning about invalid kind, got issues: %v", report.Checks)
	}
}

func TestDoctor_DaemonNotRunning(t *testing.T) {
	// given — no PID file
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then
	if !report.DaemonStatus.Checked {
		t.Error("daemon status should be checked")
	}
	if report.DaemonStatus.Running {
		t.Error("daemon should not be running")
	}
}

func TestDoctor_SkillsRefValidation(t *testing.T) {
	if !session.ExportSkillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — SKILL.md with name not matching directory (Agent Skills spec violation)
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)

	sendableDir := filepath.Join(repoDir, ".siren", "skills", "dmail-sendable")
	for _, dir := range []string{
		filepath.Join(repoDir, ".siren", "outbox"),
		filepath.Join(repoDir, ".siren", "inbox"),
		sendableDir,
		stateDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// name field doesn't match directory name — non-compliant with Agent Skills spec
	writeSkillFile(t, filepath.Join(sendableDir, "SKILL.md"),
		"---\nname: wrong-name\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: specification\n---\n")

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should have a warning from skills-ref validation
	hasSpecWarn := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckWarn && strings.Contains(issue.Message, "skills-ref") {
			hasSpecWarn = true
		}
	}
	if !hasSpecWarn {
		t.Errorf("expected skills-ref validation warning for non-compliant SKILL.md, got issues: %v", report.Checks)
	}
}

func TestFormatDoctorJSON_Parseable(t *testing.T) {
	// given — a DoctorReport with mixed issues
	report := domain.DoctorReport{
		Healthy: true,
		Checks: []domain.DoctorCheck{
			{Name: "repo/.siren", Status: domain.CheckOK, Message: "OK"},
			{Name: "repo/.expedition", Status: domain.CheckFixed, Message: "Created outbox"},
		},
		Endpoints: []domain.EndpointHealth{
			{Repo: "/tmp/repo", Dir: ".siren", Produces: []string{"specification"}, OK: true},
		},
		DaemonStatus: domain.DaemonHealthStatus{Checked: true, Running: false},
	}

	// when
	data, err := session.FormatDoctorJSON(report)

	// then — must be valid JSON
	if err != nil {
		t.Fatalf("FormatDoctorJSON: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, string(data))
	}
	// Should have top-level keys
	if _, ok := parsed["healthy"]; !ok {
		t.Error("missing 'healthy' key in JSON output")
	}
	if _, ok := parsed["checks"]; !ok {
		t.Error("missing 'checks' key in JSON output")
	}
}

func TestDoctor_IncludesSuccessRate(t *testing.T) {
	// given — a state dir with delivery.log containing recent entries
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	logContent := fmt.Sprintf("%s DELIVERED file1.md\n%s DELIVERED file2.md\n%s FAILED file3.md\n",
		now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(stateDir, "delivery.log"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should include a success-rate issue with correct stats
	var found bool
	for _, issue := range report.Checks {
		if issue.Name == "success-rate" && issue.Status == domain.CheckOK {
			found = true
			if !strings.Contains(issue.Message, "66.7%") || !strings.Contains(issue.Message, "(2/3)") {
				t.Errorf("unexpected success-rate message: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected success-rate issue in doctor report, got: %v", report.Checks)
	}
}

func TestDoctor_SuccessRate_NoDeliveries(t *testing.T) {
	// given — a state dir with no delivery.log
	stateDir := t.TempDir()
	cfg := &domain.Config{}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should still include success-rate with "no deliveries"
	var found bool
	for _, issue := range report.Checks {
		if issue.Name == "success-rate" {
			found = true
			if issue.Message != "no deliveries" {
				t.Errorf("expected 'no deliveries', got %q", issue.Message)
			}
		}
	}
	if !found {
		t.Error("expected success-rate issue even with no deliveries")
	}
}

func TestDoctor_StalePIDFile(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a PID file with a PID that definitely doesn't exist
	// Use PID 999999999 which almost certainly isn't running
	pidPath := filepath.Join(stateDir, "watch.pid")
	if err := os.WriteFile(pidPath, []byte("999999999"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — daemon should NOT be reported as running (stale PID)
	if report.DaemonStatus.Running {
		t.Error("daemon should not be reported as running with stale PID")
	}
}

func TestDoctor_MissingRepoPath_HintReferencesConfigYAML(t *testing.T) {
	// given — config references a non-existent repository path
	stateDir := t.TempDir()

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: "/nonexistent/repo/path",
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — hint should reference config.yaml
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckFail && strings.Contains(issue.Message, "does not exist") {
			if !strings.Contains(issue.Hint, "config.yaml") {
				t.Errorf("hint should reference config.yaml, got: %q", issue.Hint)
			}
		}
	}
}

func TestDoctor_WarnsWhenResolvedStateMissing(t *testing.T) {
	// given — state dir exists but resolved.yaml does not
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// Create endpoint dir so we don't get other errors
	if err := os.MkdirAll(filepath.Join(repoDir, ".siren"), 0755); err != nil {
		t.Fatal(err)
	}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should have a warning about missing resolved state
	hasResolvedWarn := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckWarn && strings.Contains(issue.Message, "resolved.yaml") {
			hasResolvedWarn = true
		}
	}
	if !hasResolvedWarn {
		t.Errorf("expected warning about missing resolved.yaml, got issues: %v", report.Checks)
	}
}

func TestDoctor_NoWarningWhenResolvedStateExists(t *testing.T) {
	// given — state dir with resolved.yaml present
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	resolvedPath := filepath.Join(runDir, domain.ResolvedStateFile)
	if err := os.WriteFile(resolvedPath, []byte("last_synced: 2026-03-08T12:00:00Z\nroutes: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{}

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should NOT have a warning about missing resolved state
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckWarn && strings.Contains(issue.Message, "resolved.yaml") {
			t.Errorf("unexpected warning about resolved.yaml when it exists: %v", issue)
		}
	}
}

func TestDoctor_SkillsRefToolchainCheck(t *testing.T) {
	// given — empty config, any environment
	cfg := &domain.Config{}
	stateDir := filepath.Join(t.TempDir(), domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	// when
	report := session.Doctor(cfg, stateDir, false, "")

	// then — should have a skills-ref related issue (ok or warn)
	hasSkillsRef := false
	for _, issue := range report.Checks {
		if issue.Name == "skills-ref" {
			hasSkillsRef = true
			break
		}
	}
	if !hasSkillsRef {
		t.Error("expected a skills-ref toolchain issue in doctor report")
	}
}

func TestDoctor_SkillsRefInstallSucceedsButNotOnPath(t *testing.T) {
	// given — uv on PATH, skills-ref NOT on PATH, no submodule
	// install succeeds but skills-ref is STILL not on PATH after install
	cfg := &domain.Config{}
	stateDir := filepath.Join(t.TempDir(), domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	cleanup := session.OverrideRepairInstallSkillsRef(func() error {
		return nil // install "succeeds"
	})
	defer cleanup()

	cleanup2 := session.OverrideLookPath(func(name string) (string, error) {
		if name == "uv" {
			return "/usr/bin/uv", nil
		}
		// skills-ref is NEVER found, even after install
		return "", exec.ErrNotFound
	})
	defer cleanup2()

	cleanup3 := session.OverrideFindSkillsRefDir(func() string {
		return "" // no submodule
	})
	defer cleanup3()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true, "")

	// then — should be WARN (not FIXED) since skills-ref is still not on PATH
	hasWarn := false
	hasFixed := false
	for _, issue := range report.Checks {
		if issue.Name == "skills-ref" {
			if issue.Status == domain.CheckWarn {
				hasWarn = true
			}
			if issue.Status == domain.CheckFixed {
				hasFixed = true
			}
		}
	}
	if hasFixed {
		t.Error("should not report FIXED when skills-ref is still not on PATH after install")
	}
	if !hasWarn {
		t.Errorf("expected WARN when skills-ref is not on PATH after install, got: %v", report.Checks)
	}
}

func TestDoctor_IsProcessAlive_WindowsFallback(t *testing.T) {
	// given — test that isProcessAlive is exported and works for current process
	pid := os.Getpid()

	// when
	alive := session.IsProcessAlive(pid)

	// then — current process should be alive
	if !alive {
		t.Error("expected current process to be alive")
	}
}

func TestDoctor_IsProcessAlive_NonExistentPID(t *testing.T) {
	// given — a PID that almost certainly doesn't exist
	pid := 999999999

	// when
	alive := session.IsProcessAlive(pid)

	// then — should not be alive
	if alive {
		t.Error("expected non-existent PID to not be alive")
	}
}

func TestDoctor_RepairDoesNotDropMissingRepoFromConfig(t *testing.T) {
	// given — config.yaml has a repo whose path does not exist on disk;
	// resolved.yaml is missing so repair triggers regeneration.
	baseDir := t.TempDir()
	stateDir := filepath.Join(baseDir, domain.StateDir)
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}

	// A repo path that does NOT exist on disk
	missingRepoPath := filepath.Join(baseDir, "repo-that-does-not-exist")

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: missingRepoPath,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// Snapshot the original repos count
	originalRepoCount := len(cfg.Repositories)

	// Do NOT override repairSyncFn — use the real default implementation
	// which should call WriteResolvedOnly (not Sync)

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true, "")

	// then — config.Repositories should NOT have been modified (repo not dropped)
	if len(cfg.Repositories) != originalRepoCount {
		t.Errorf("repair dropped repo from config: got %d repos, want %d",
			len(cfg.Repositories), originalRepoCount)
	}
	if cfg.Repositories[0].Path != missingRepoPath {
		t.Errorf("repair changed repo path: got %q, want %q",
			cfg.Repositories[0].Path, missingRepoPath)
	}

	// resolved.yaml should have been generated
	resolvedPath := filepath.Join(runDir, domain.ResolvedStateFile)
	if _, err := os.Stat(resolvedPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("resolved.yaml should have been generated by repair")
	}

	// report should show fixed for resolved.yaml
	hasFixed := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckFixed && strings.Contains(issue.Message, "resolved") {
			hasFixed = true
		}
	}
	if !hasFixed {
		t.Errorf("expected fixed issue for resolved.yaml, got: %v", report.Checks)
	}
}

func TestDoctor_RepairStalePID_AlsoRemovesWatchStarted(t *testing.T) {
	// given — stale PID file and watch.started both exist
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	pidPath := filepath.Join(stateDir, "watch.pid")
	os.WriteFile(pidPath, []byte("999999999"), 0644)

	startedPath := filepath.Join(stateDir, "watch.started")
	os.WriteFile(startedPath, []byte("2026-03-14T10:00:00Z"), 0644)

	cfg := &domain.Config{}

	// when — repair=true
	_ = session.Doctor(cfg, stateDir, true, "")

	// then — both watch.pid and watch.started should be removed
	if _, err := os.Stat(pidPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected watch.pid to be removed after stale PID repair")
	}
	if _, err := os.Stat(startedPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected watch.started to be removed after stale PID repair")
	}
}

func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDoctor_EventStoreCorruptLines(t *testing.T) {
	// given: state dir with corrupt event lines
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0755)

	// Valid event + corrupt line + another valid event
	validEvent := `{"type":"delivery.completed","data":{"path":"/test"},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	corruptLine := `{not valid json`
	os.WriteFile(filepath.Join(eventsDir, "2026-04-08.jsonl"),
		[]byte(validEvent+"\n"+corruptLine+"\n"+validEvent+"\n"), 0644)

	cfg := &domain.Config{}
	report := session.Doctor(cfg, stateDir, false, "")

	// then: report should contain a warning about corrupt lines
	found := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckWarn && strings.Contains(issue.Message, "corrupt line") {
			found = true
			if !strings.Contains(issue.Message, "1 corrupt line") {
				t.Errorf("expected 1 corrupt line, got: %q", issue.Message)
			}
		}
	}
	if !found {
		t.Error("expected warn issue about corrupt event lines, got none")
	}
}

func TestFormatDoctorJSON_StatusLabelsAreKnown(t *testing.T) {
	report := domain.DoctorReport{
		Healthy: true,
		Checks: []domain.DoctorCheck{
			{Name: "test-ok", Status: domain.CheckOK, Message: "ok"},
			{Name: "test-fail", Status: domain.CheckFail, Message: "fail"},
			{Name: "test-warn", Status: domain.CheckWarn, Message: "warn"},
			{Name: "test-skip", Status: domain.CheckSkip, Message: "skip"},
			{Name: "test-fix", Status: domain.CheckFixed, Message: "fix"},
		},
	}
	data, err := session.FormatDoctorJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{"OK": true, "FAIL": true, "SKIP": true, "WARN": true, "FIX": true}
	var parsed struct {
		Checks []struct{ Status string } `json:"checks"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, c := range parsed.Checks {
		if !known[c.Status] {
			t.Errorf("unknown status label in JSON: %q", c.Status)
		}
	}
}

func TestDoctor_EventStoreClean(t *testing.T) {
	// given: state dir with clean event files
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0755)

	validEvent := `{"type":"delivery.completed","data":{"path":"/test"},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	os.WriteFile(filepath.Join(eventsDir, "2026-04-08.jsonl"),
		[]byte(validEvent+"\n"), 0644)

	cfg := &domain.Config{}
	report := session.Doctor(cfg, stateDir, false, "")

	// then: report should have OK for event store
	found := false
	for _, issue := range report.Checks {
		if issue.Status == domain.CheckOK && strings.Contains(issue.Message, "event store OK") {
			found = true
		}
	}
	if !found {
		t.Error("expected OK issue for clean event store, got none")
	}
}
