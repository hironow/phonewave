package session_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestStatus_DaemonStopped(t *testing.T) {
	// given — no PID file, a config with some endpoints
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
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
				},
			},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}},
		},
	}

	// when
	status := session.Status(cfg, stateDir)

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
	// given — enqueue items into SQLite error queue
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	eq, err := session.NewErrorQueueStore(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"failed-001.md", "failed-002.md"} {
		if err := eq.Enqueue(name, []byte("error"), domain.ErrorMetadata{
			Kind:  "specification",
			Error: "no route",
		}); err != nil {
			t.Fatal(err)
		}
	}
	eq.Close()

	cfg := &domain.Config{}

	// when
	status := session.Status(cfg, stateDir)

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
%s FAILED    kind=design-feedback from=/outbox/fb.md reason=no route
%s RETRIED   kind=specification from=/outbox/spec.md to=/inbox/spec.md
%s DELIVERED kind=report from=/outbox/old.md to=/inbox/old.md
`, recent, recent, recent, old)

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	stats := session.ParseDeliveryStats(stateDir)

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
	stats := session.ParseDeliveryStats(stateDir)

	// then
	if stats.Delivered != 0 || stats.Failed != 0 || stats.Retried != 0 {
		t.Errorf("empty log stats should all be 0, got delivered=%d failed=%d retried=%d",
			stats.Delivered, stats.Failed, stats.Retried)
	}
}

func TestStatus_Uptime(t *testing.T) {
	// given — a running daemon with watch.started file
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a watch.started file from 2 hours ago
	startTime := time.Now().UTC().Add(-2 * time.Hour)
	startedPath := filepath.Join(stateDir, "watch.started")
	if err := os.WriteFile(startedPath, []byte(startTime.Format(time.RFC3339)), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{}

	// when
	status := session.Status(cfg, stateDir)

	// then — uptime should be approximately 2 hours
	if status.Uptime < 1*time.Hour || status.Uptime > 3*time.Hour {
		t.Errorf("uptime = %v, want ~2h", status.Uptime)
	}
}

func TestStatus_DeliveryStats(t *testing.T) {
	// given — a state dir with delivery.log
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	recent := now.Add(-30 * time.Minute).Format(time.RFC3339)
	logContent := fmt.Sprintf(`%s DELIVERED kind=specification from=/outbox/spec.md to=/inbox/spec.md
%s DELIVERED kind=report from=/outbox/rpt.md to=/inbox/rpt.md
%s FAILED    kind=design-feedback from=/outbox/fb.md reason=no route
`, recent, recent, recent)

	if err := os.WriteFile(filepath.Join(stateDir, "delivery.log"), []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.Config{}

	// when
	status := session.Status(cfg, stateDir)

	// then
	if status.DeliveredCount24h != 2 {
		t.Errorf("delivered 24h = %d, want 2", status.DeliveredCount24h)
	}
	if status.FailedCount24h != 1 {
		t.Errorf("failed 24h = %d, want 1", status.FailedCount24h)
	}
	// SuccessRate24h = 2 delivered / (2 delivered + 1 failed) ≈ 0.6667
	wantRate := 2.0 / 3.0
	if status.SuccessRate24h < wantRate-0.01 || status.SuccessRate24h > wantRate+0.01 {
		t.Errorf("success rate 24h = %f, want ~%f", status.SuccessRate24h, wantRate)
	}
}

func TestStatus_SuccessRate_NoDeliveries(t *testing.T) {
	// given — no delivery log
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	os.MkdirAll(stateDir, 0755)
	cfg := &domain.Config{}

	// when
	status := session.Status(cfg, stateDir)

	// then — 0 deliveries → 0.0 success rate
	if status.SuccessRate24h != 0.0 {
		t.Errorf("success rate 24h = %f, want 0.0 (no deliveries)", status.SuccessRate24h)
	}
}
