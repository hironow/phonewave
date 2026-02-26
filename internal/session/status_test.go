package session

import (
	phonewave "github.com/hironow/phonewave"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatus_DaemonStopped(t *testing.T) {
	// given — no PID file, a config with some endpoints
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, phonewave.StateDir)
	if err := os.MkdirAll(filepath.Join(stateDir, "errors"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &phonewave.Config{
		Repositories: []phonewave.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []phonewave.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
				},
			},
		},
		Routes: []phonewave.RouteConfig{
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
	stateDir := filepath.Join(repoDir, phonewave.StateDir)
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

	cfg := &phonewave.Config{}

	// when
	status := Status(cfg, stateDir)

	// then
	if status.PendingErrors != 2 {
		t.Errorf("pending errors = %d, want 2", status.PendingErrors)
	}
}

func TestParseDeliveryStats_CountsLast24Hours(t *testing.T) {
	// given — a delivery log with entries from different times
	stateDir := t.TempDir()
	logPath := filepath.Join(stateDir, "delivery.log")

	now := time.Now().UTC()
	recent := now.Add(-1 * time.Hour).Format(time.RFC3339)
	old := now.Add(-25 * time.Hour).Format(time.RFC3339)

	content := fmt.Sprintf(`%s DELIVERED kind=specification from=/outbox/spec.md to=/inbox/spec.md
%s FAILED    kind=feedback from=/outbox/fb.md reason=no route
%s RETRIED   kind=specification from=/outbox/spec.md to=/inbox/spec.md
%s DELIVERED kind=report from=/outbox/old.md to=/inbox/old.md
`, recent, recent, recent, old)

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	stats := ParseDeliveryStats(stateDir)

	// then — only recent entries should count
	if stats.Delivered != 1 {
		t.Errorf("delivered = %d, want 1 (only recent)", stats.Delivered)
	}
	if stats.Failed != 1 {
		t.Errorf("failed = %d, want 1", stats.Failed)
	}
	if stats.Retried != 1 {
		t.Errorf("retried = %d, want 1", stats.Retried)
	}
}

func TestParseDeliveryStats_EmptyLog(t *testing.T) {
	// given — no delivery log
	stateDir := t.TempDir()

	// when
	stats := ParseDeliveryStats(stateDir)

	// then
	if stats.Delivered != 0 || stats.Failed != 0 || stats.Retried != 0 {
		t.Errorf("empty log stats should all be 0, got delivered=%d failed=%d retried=%d",
			stats.Delivered, stats.Failed, stats.Retried)
	}
}

func TestStatus_Uptime(t *testing.T) {
	// given — a running daemon with watch.started file
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, phonewave.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a watch.started file from 2 hours ago
	startTime := time.Now().UTC().Add(-2 * time.Hour)
	startedPath := filepath.Join(stateDir, "watch.started")
	if err := os.WriteFile(startedPath, []byte(startTime.Format(time.RFC3339)), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &phonewave.Config{}

	// when
	status := Status(cfg, stateDir)

	// then — uptime should be approximately 2 hours
	if status.Uptime < 1*time.Hour || status.Uptime > 3*time.Hour {
		t.Errorf("uptime = %v, want ~2h", status.Uptime)
	}
}

func TestStatus_DeliveryStats(t *testing.T) {
	// given — a state dir with delivery.log
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, phonewave.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	recent := now.Add(-30 * time.Minute).Format(time.RFC3339)
	logContent := fmt.Sprintf(`%s DELIVERED kind=specification from=/outbox/spec.md to=/inbox/spec.md
%s DELIVERED kind=report from=/outbox/rpt.md to=/inbox/rpt.md
%s FAILED    kind=feedback from=/outbox/fb.md reason=no route
`, recent, recent, recent)

	if err := os.WriteFile(filepath.Join(stateDir, "delivery.log"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &phonewave.Config{}

	// when
	status := Status(cfg, stateDir)

	// then
	if status.DeliveredCount24h != 2 {
		t.Errorf("delivered 24h = %d, want 2", status.DeliveredCount24h)
	}
	if status.FailedCount24h != 1 {
		t.Errorf("failed 24h = %d, want 1", status.FailedCount24h)
	}
}
