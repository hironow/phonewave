package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestNewInitCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	paths, _ := domain.NewNonEmptyRepoPaths([]domain.RepoPath{rp})
	cp, _ := domain.NewConfigPath("/tmp/config.yaml")

	cmd := domain.NewInitCommand(paths, cp)

	if len(cmd.RepoPaths().Strings()) != 1 {
		t.Errorf("expected 1 repo path, got %d", len(cmd.RepoPaths().Strings()))
	}
	if cmd.ConfigPath().String() != "/tmp/config.yaml" {
		t.Errorf("expected /tmp/config.yaml, got %q", cmd.ConfigPath().String())
	}
}

func TestNewRunDaemonCommand(t *testing.T) {
	ri, _ := domain.NewRetryInterval(60 * time.Second)
	mr, _ := domain.NewMaxRetries(10)

	cmd := domain.NewRunDaemonCommand(true, false, ri, mr)

	if !cmd.Verbose() {
		t.Error("expected Verbose to be true")
	}
	if cmd.DryRun() {
		t.Error("expected DryRun to be false")
	}
	if cmd.RetryDuration() != 60*time.Second {
		t.Errorf("expected 60s, got %v", cmd.RetryDuration())
	}
	if cmd.MaxRetriesInt() != 10 {
		t.Errorf("expected 10, got %d", cmd.MaxRetriesInt())
	}
}

func TestNewAddRepoCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewAddRepoCommand(rp)

	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.RepoPath().String())
	}
}

func TestNewRemoveRepoCommand(t *testing.T) {
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewRemoveRepoCommand(rp)

	if cmd.RepoPath().String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", cmd.RepoPath().String())
	}
}
