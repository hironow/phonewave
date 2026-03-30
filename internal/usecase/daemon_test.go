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

func TestSetupAndRunDaemon_EmptyOutbox(t *testing.T) {
	// given: valid command but no outbox directories
	ri, _ := domain.NewRetryInterval(60 * time.Second)
	mr, _ := domain.NewMaxRetries(10)
	cmd := domain.NewRunDaemonCommand(false, false, ri, mr, domain.DefaultIdleTimeout)
	logger := platform.NewLogger(io.Discard, false)

	// when: NopDaemonRunner has 0 outbox count
	err := SetupAndRunDaemon(context.Background(), cmd, logger, nil, port.NopDaemonRunner{})

	// then: returns nil (no outboxes to watch)
	if err != nil {
		t.Fatalf("expected nil error for empty outbox, got %v", err)
	}
}

func TestSetupAndRunDaemon_MissingConfig(t *testing.T) {
	// given: valid command but nonexistent config
	ri, _ := domain.NewRetryInterval(60 * time.Second)
	mr, _ := domain.NewMaxRetries(10)
	cmd := domain.NewRunDaemonCommand(false, false, ri, mr, domain.DefaultIdleTimeout)
	logger := platform.NewLogger(io.Discard, false)

	// when: factory should fail with missing config
	_, err := session.NewDaemonRunner(cmd, "/nonexistent/config.yaml", "/tmp", logger)

	// then
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestSetupAndRunDaemon_RejectsConcurrentStart(t *testing.T) {
	// given: a temp project directory with a valid config and a pre-held lock
	baseDir := t.TempDir()
	repoDir := t.TempDir()

	// Build a minimal config with one producing endpoint so CollectOutboxDirs
	// returns a non-empty slice (otherwise SetupAndRunDaemon returns nil early).
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

	// Pre-acquire the daemon lock (simulating an already-running daemon)
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

	// when: factory should fail with lock already held
	// baseDir param is now the state dir (configBase returns .phonewave/)
	_, err = session.NewDaemonRunner(daemonCmd, configPath, stateDirPath, logger)

	// then: must fail with "already running"
	if err == nil {
		t.Fatal("expected error when daemon lock is already held")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' in error, got: %v", err)
	}
}
