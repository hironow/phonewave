package phonewave

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatus_DaemonStopped(t *testing.T) {
	// given — no PID file, a config with some endpoints
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)
	if err := os.MkdirAll(filepath.Join(stateDir, "errors"), 0755); err != nil {
		t.Fatal(err)
	}

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
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}},
		},
	}

	// when
	status := Status(cfg, stateDir)

	// then
	if status.DaemonRunning {
		t.Error("daemon should not be running")
	}
	if status.OutboxCount != 2 {
		t.Errorf("outbox count = %d, want 2", status.OutboxCount)
	}
	if status.RouteCount != 1 {
		t.Errorf("route count = %d, want 1", status.RouteCount)
	}
	if status.RepoCount != 1 {
		t.Errorf("repo count = %d, want 1", status.RepoCount)
	}
	if status.PendingErrors != 0 {
		t.Errorf("pending errors = %d, want 0", status.PendingErrors)
	}
}

func TestStatus_PendingErrors(t *testing.T) {
	// given — some files in errors/ directory
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, StateDir)
	errorsDir := filepath.Join(stateDir, "errors")
	if err := os.MkdirAll(errorsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write two error files
	for _, name := range []string{"failed-001.md", "failed-002.md"} {
		if err := os.WriteFile(filepath.Join(errorsDir, name), []byte("error"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &Config{}

	// when
	status := Status(cfg, stateDir)

	// then
	if status.PendingErrors != 2 {
		t.Errorf("pending errors = %d, want 2", status.PendingErrors)
	}
}
