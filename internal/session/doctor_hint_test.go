package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestDoctor_MissingRepoPath_HasHint(t *testing.T) {
	// given
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
	report := session.Doctor(cfg, stateDir, false)

	// then
	for _, issue := range report.Issues {
		if issue.Severity == "error" && strings.Contains(issue.Message, "does not exist") {
			if issue.Hint == "" {
				t.Error("expected hint for missing repo path error")
			}
			if !strings.Contains(issue.Hint, "config.yaml") {
				t.Errorf("hint should mention config.yaml, got: %s", issue.Hint)
			}
			return
		}
	}
	t.Error("expected error issue for missing repo path")
}

func TestDoctor_MissingEndpointDir_HasHint(t *testing.T) {
	// given — repo exists but endpoint dir does not
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
					{Dir: ".nonexistent-ep", Produces: []string{"specification"}},
				},
			},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false)

	// then
	for _, issue := range report.Issues {
		if issue.Severity == "error" && strings.Contains(issue.Message, "Endpoint directory missing") {
			if issue.Hint == "" {
				t.Error("expected hint for missing endpoint directory error")
			}
			return
		}
	}
	t.Error("expected error issue for missing endpoint dir")
}

func TestDoctor_OrphanedKind_HasHint(t *testing.T) {
	// given — config with an unconsumed kind
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	for _, dir := range []string{
		filepath.Join(repoDir, ".siren", "outbox"),
		filepath.Join(repoDir, ".siren", "inbox"),
		stateDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{}},
				},
			},
		},
	}

	// when
	report := session.Doctor(cfg, stateDir, false)

	// then
	for _, issue := range report.Issues {
		if issue.Severity == "warn" && strings.Contains(issue.Message, "Orphaned") {
			if issue.Hint == "" {
				t.Error("expected hint for orphaned kind warning")
			}
			return
		}
	}
	t.Error("expected warn issue for orphaned kind")
}

func TestAddErrorWithHint(t *testing.T) {
	// given
	report := domain.DoctorReport{Healthy: true}

	// when
	report.AddErrorWithHint("ep", "msg", "the hint")

	// then
	if report.Healthy {
		t.Error("expected unhealthy")
	}
	if len(report.Issues) != 1 {
		t.Fatalf("issues len = %d, want 1", len(report.Issues))
	}
	if report.Issues[0].Hint != "the hint" {
		t.Errorf("hint = %q, want %q", report.Issues[0].Hint, "the hint")
	}
	if report.Issues[0].Severity != "error" {
		t.Errorf("severity = %q, want %q", report.Issues[0].Severity, "error")
	}
}

func TestAddWarnWithHint(t *testing.T) {
	// given
	report := domain.DoctorReport{Healthy: true}

	// when
	report.AddWarnWithHint("ep", "msg", "warn hint")

	// then
	if !report.Healthy {
		t.Error("warn should not mark unhealthy")
	}
	if len(report.Issues) != 1 {
		t.Fatalf("issues len = %d, want 1", len(report.Issues))
	}
	if report.Issues[0].Hint != "warn hint" {
		t.Errorf("hint = %q, want %q", report.Issues[0].Hint, "warn hint")
	}
}

func TestFormatDoctorJSON_IncludesHint(t *testing.T) {
	// given
	report := domain.DoctorReport{
		Healthy: true,
		Issues: []domain.DoctorIssue{
			{Endpoint: "ep", Message: "msg", Severity: "error", Hint: "do this"},
		},
		DaemonStatus: domain.DaemonHealthStatus{Checked: true},
	}

	// when
	data, err := session.FormatDoctorJSON(report)

	// then
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"hint"`) {
		t.Errorf("JSON should contain hint field: %s", string(data))
	}
	if !strings.Contains(string(data), "do this") {
		t.Errorf("JSON should contain hint value: %s", string(data))
	}
}

func TestFormatDoctorJSON_OmitsEmptyHint(t *testing.T) {
	// given
	report := domain.DoctorReport{
		Healthy: true,
		Issues: []domain.DoctorIssue{
			{Endpoint: "ep", Message: "OK", Severity: "ok"},
		},
		DaemonStatus: domain.DaemonHealthStatus{Checked: true},
	}

	// when
	data, err := session.FormatDoctorJSON(report)

	// then
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"hint"`) {
		t.Errorf("JSON should omit empty hint field: %s", string(data))
	}
}
