package session_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

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
	report := session.Doctor(cfg, stateDir)

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
	report := session.Doctor(cfg, stateDir)

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
	report := session.Doctor(cfg, stateDir)

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
	report := session.Doctor(cfg, stateDir)

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
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{}

	// when
	report := session.Doctor(cfg, stateDir)

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
	report := session.Doctor(cfg, stateDir)

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
	report := domain.DoctorReport{
		Healthy: true,
		Issues: []domain.DoctorIssue{
			{Endpoint: "repo/.siren", Message: "OK", Severity: "ok"},
			{Endpoint: "repo/.expedition", Message: "Created outbox", Severity: "fixed"},
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
	if _, ok := parsed["issues"]; !ok {
		t.Error("missing 'issues' key in JSON output")
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
	report := session.Doctor(cfg, stateDir)

	// then — should include a success-rate issue with correct stats
	var found bool
	for _, issue := range report.Issues {
		if issue.Endpoint == "success-rate" && issue.Severity == "ok" {
			found = true
			if !strings.Contains(issue.Message, "66.7%") || !strings.Contains(issue.Message, "(2/3)") {
				t.Errorf("unexpected success-rate message: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected success-rate issue in doctor report, got: %v", report.Issues)
	}
}

func TestDoctor_SuccessRate_NoDeliveries(t *testing.T) {
	// given — a state dir with no delivery.log
	stateDir := t.TempDir()
	cfg := &domain.Config{}

	// when
	report := session.Doctor(cfg, stateDir)

	// then — should still include success-rate with "no deliveries"
	var found bool
	for _, issue := range report.Issues {
		if issue.Endpoint == "success-rate" {
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
	report := session.Doctor(cfg, stateDir)

	// then — daemon should NOT be reported as running (stale PID)
	if report.DaemonStatus.Running {
		t.Error("daemon should not be reported as running with stale PID")
	}
}

func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
