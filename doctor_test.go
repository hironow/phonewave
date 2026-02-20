package phonewave

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctor_HealthyEcosystem(t *testing.T) {
	// given — a fully set up repo with all dirs and SKILL.md files
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)

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

	// Write SKILL.md files
	writeSkillFile(t, filepath.Join(repoDir, ".siren", "skills", "dmail-sendable", "SKILL.md"),
		"---\nname: dmail-sendable\nproduces:\n  - kind: specification\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".siren", "skills", "dmail-readable", "SKILL.md"),
		"---\nname: dmail-readable\nconsumes:\n  - kind: feedback\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".expedition", "skills", "dmail-sendable", "SKILL.md"),
		"---\nname: dmail-sendable\nproduces:\n  - kind: report\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".expedition", "skills", "dmail-readable", "SKILL.md"),
		"---\nname: dmail-readable\nconsumes:\n  - kind: specification\n---\n")

	cfg := &Config{
		Repositories: []RepoConfig{
			{
				Path: repoDir,
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
				},
			},
		},
		Routes: []RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	// when
	report := Doctor(cfg, stateDir)

	// then
	if !report.Healthy {
		t.Errorf("expected healthy ecosystem, got issues: %v", report.Issues)
	}
	if len(report.Endpoints) != 2 {
		t.Errorf("endpoints = %d, want 2", len(report.Endpoints))
	}
}

func TestDoctor_MissingDirs(t *testing.T) {
	// given — repo path exists but outbox/inbox dirs are missing
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create endpoint dir but NOT outbox/inbox
	if err := os.MkdirAll(filepath.Join(repoDir, ".siren"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Repositories: []RepoConfig{
			{
				Path: repoDir,
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := Doctor(cfg, stateDir)

	// then — should have warnings about missing dirs but auto-create them
	hasCreated := false
	for _, issue := range report.Issues {
		if issue.Severity == "fixed" {
			hasCreated = true
		}
	}
	if !hasCreated {
		t.Error("expected 'fixed' issues for auto-created directories")
	}

	// outbox and inbox should now exist
	if _, err := os.Stat(filepath.Join(repoDir, ".siren", "outbox")); os.IsNotExist(err) {
		t.Error("outbox should have been auto-created")
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".siren", "inbox")); os.IsNotExist(err) {
		t.Error("inbox should have been auto-created")
	}
}

func TestDoctor_MissingRepoPath(t *testing.T) {
	// given — config references a non-existent repository path
	stateDir := t.TempDir()

	cfg := &Config{
		Repositories: []RepoConfig{
			{
				Path: "/nonexistent/repo/path",
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := Doctor(cfg, stateDir)

	// then
	if report.Healthy {
		t.Error("expected unhealthy with missing repo path")
	}
	hasError := false
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected error-level issue for missing repo path")
	}
}

func TestDoctor_DaemonNotRunning(t *testing.T) {
	// given — no PID file
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}

	// when
	report := Doctor(cfg, stateDir)

	// then
	if !report.DaemonStatus.Checked {
		t.Error("daemon status should be checked")
	}
	if report.DaemonStatus.Running {
		t.Error("daemon should not be running")
	}
}

func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
