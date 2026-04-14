package usecase

// white-box-reason: usecase internals: tests unexported session adapter wiring for daemon

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
	"github.com/hironow/phonewave/internal/usecase/port"
)

func TestPrepareDaemonRunner_DoesNotPanic(t *testing.T) {
	// given
	logger := platform.NewLogger(io.Discard, false)

	// when: should not panic even with NopDaemonRunner
	PrepareDaemonRunner(context.Background(), logger, nil, port.NopDaemonRunner{})

	// then: no panic = success
}

func TestDaemonRunner_MissingConfig(t *testing.T) {
	ri, _ := domain.NewRetryInterval(60 * time.Second)
	mr, _ := domain.NewMaxRetries(10)
	cmd := domain.NewRunDaemonCommand(false, false, ri, mr, domain.DefaultIdleTimeout)
	logger := platform.NewLogger(io.Discard, false)

	_, err := session.NewDaemonRunner(cmd, "/nonexistent/config.yaml", "/tmp", logger)

	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestDaemonRunner_RejectsConcurrentStart(t *testing.T) {
	baseDir := t.TempDir()
	repoDir := t.TempDir()

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{
						Dir:      ".siren",
						Produces: []string{"specification"},
						Consumes: []string{"design-feedback"},
					},
				},
			},
		},
		Routes: []domain.RouteConfig{
			{
				Kind:     "specification",
				From:     ".siren/outbox",
				To:       []string{".siren/inbox"},
				Scope:    "same_repository",
				RepoPath: repoDir,
			},
		},
	}

	stateDirPath := filepath.Join(baseDir, domain.StateDir)
	if err := os.MkdirAll(stateDirPath, 0o755); err != nil {
		t.Fatalf("MkdirAll stateDir: %v", err)
	}
	configPath := filepath.Join(stateDirPath, domain.ConfigFile)
	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	runDir := filepath.Join(stateDirPath, ".run")
	unlock, err := session.TryLockDaemon(runDir)
	if err != nil {
		t.Fatalf("pre-acquire lock: %v", err)
	}
	defer unlock()

	ri, _ := domain.NewRetryInterval(60 * time.Second)
	mr, _ := domain.NewMaxRetries(10)
	daemonCmd := domain.NewRunDaemonCommand(false, false, ri, mr, domain.DefaultIdleTimeout)
	logger := platform.NewLogger(io.Discard, false)

	_, err = session.NewDaemonRunner(daemonCmd, configPath, stateDirPath, logger)

	if err == nil {
		t.Fatal("expected error when daemon lock is already held")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' in error, got: %v", err)
	}
}
