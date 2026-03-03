package usecase

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestSetupAndRunDaemon_InvalidCommand(t *testing.T) {
	// given: RetryInterval <= 0
	cmd := domain.RunDaemonCommand{
		RetryInterval: 0,
		MaxRetries:    10,
	}
	logger := domain.NewLogger(io.Discard, false)

	// when
	err := SetupAndRunDaemon(context.Background(), cmd, "/nonexistent/config.yaml", "/tmp", logger)

	// then
	if err == nil {
		t.Fatal("expected validation error for zero RetryInterval")
	}
}

func TestSetupAndRunDaemon_MissingConfig(t *testing.T) {
	// given: valid command but nonexistent config
	cmd := domain.RunDaemonCommand{
		RetryInterval: 60 * time.Second,
		MaxRetries:    10,
	}
	logger := domain.NewLogger(io.Discard, false)

	// when
	err := SetupAndRunDaemon(context.Background(), cmd, "/nonexistent/config.yaml", "/tmp", logger)

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
						Consumes: []string{"feedback"},
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

	cmd := domain.RunDaemonCommand{
		RetryInterval: 60 * time.Second,
		MaxRetries:    10,
	}
	logger := domain.NewLogger(io.Discard, false)

	// when
	err = SetupAndRunDaemon(context.Background(), cmd, configPath, baseDir, logger)

	// then: must fail with "already running"
	if err == nil {
		t.Fatal("expected error when daemon lock is already held")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' in error, got: %v", err)
	}
}
