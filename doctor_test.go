package phonewave

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	// Write SKILL.md files (metadata-nested format, schema v1)
	writeSkillFile(t, filepath.Join(repoDir, ".siren", "skills", "dmail-sendable", "SKILL.md"),
		"---\nname: dmail-sendable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: specification\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".siren", "skills", "dmail-readable", "SKILL.md"),
		"---\nname: dmail-readable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n    - kind: feedback\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".expedition", "skills", "dmail-sendable", "SKILL.md"),
		"---\nname: dmail-sendable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: report\n---\n")
	writeSkillFile(t, filepath.Join(repoDir, ".expedition", "skills", "dmail-readable", "SKILL.md"),
		"---\nname: dmail-readable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n    - kind: specification\n---\n")

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

func TestDoctor_InvalidKindInSkillMD(t *testing.T) {
	// given — SKILL.md with an invalid kind
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)

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

	// then — should have a warning about invalid kind
	hasKindWarn := false
	for _, issue := range report.Issues {
		if issue.Severity == "warn" && strings.Contains(issue.Message, "invalid D-Mail kind") {
			hasKindWarn = true
		}
	}
	if !hasKindWarn {
		t.Errorf("expected warning about invalid kind, got issues: %v", report.Issues)
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

func TestDoctor_SkillsRefValidation(t *testing.T) {
	if !skillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — SKILL.md with name not matching directory (Agent Skills spec violation)
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)

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

	// then — should have a warning from skills-ref validation
	hasSpecWarn := false
	for _, issue := range report.Issues {
		if issue.Severity == "warn" && strings.Contains(issue.Message, "skills-ref") {
			hasSpecWarn = true
		}
	}
	if !hasSpecWarn {
		t.Errorf("expected skills-ref validation warning for non-compliant SKILL.md, got issues: %v", report.Issues)
	}
}

func TestFormatDoctorJSON_Parseable(t *testing.T) {
	// given — a DoctorReport with mixed issues
	report := DoctorReport{
		Healthy: true,
		Issues: []DoctorIssue{
			{Endpoint: "repo/.siren", Message: "OK", Severity: "ok"},
			{Endpoint: "repo/.expedition", Message: "Created outbox", Severity: "fixed"},
		},
		Endpoints: []EndpointHealth{
			{Repo: "/tmp/repo", Dir: ".siren", Produces: []string{"specification"}, OK: true},
		},
		DaemonStatus: DaemonHealthStatus{Checked: true, Running: false},
	}

	// when
	data, err := FormatDoctorJSON(report)

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
	if _, ok := parsed["issues"]; !ok {
		t.Error("missing 'issues' key in JSON output")
	}
}

func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
