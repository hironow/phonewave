package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestNewRepoPath_Valid(t *testing.T) {
	rp, err := domain.NewRepoPath("/tmp/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.String() != "/tmp/repo" {
		t.Errorf("expected /tmp/repo, got %q", rp.String())
	}
}

func TestNewRepoPath_RejectsEmpty(t *testing.T) {
	_, err := domain.NewRepoPath("")
	if err == nil {
		t.Fatal("expected error for empty RepoPath")
	}
}

func TestNewConfigPath_Valid(t *testing.T) {
	cp, err := domain.NewConfigPath("/tmp/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cp.String() != "/tmp/config.yaml" {
		t.Errorf("expected /tmp/config.yaml, got %q", cp.String())
	}
}

func TestNewConfigPath_RejectsEmpty(t *testing.T) {
	_, err := domain.NewConfigPath("")
	if err == nil {
		t.Fatal("expected error for empty ConfigPath")
	}
}

func TestNewNonEmptyRepoPaths_Valid(t *testing.T) {
	rp1, _ := domain.NewRepoPath("/tmp/repo1")
	rp2, _ := domain.NewRepoPath("/tmp/repo2")
	paths, err := domain.NewNonEmptyRepoPaths([]domain.RepoPath{rp1, rp2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths.Paths()) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths.Paths()))
	}
	strs := paths.Strings()
	if strs[0] != "/tmp/repo1" || strs[1] != "/tmp/repo2" {
		t.Errorf("unexpected strings: %v", strs)
	}
}

func TestNewNonEmptyRepoPaths_RejectsEmpty(t *testing.T) {
	_, err := domain.NewNonEmptyRepoPaths(nil)
	if err == nil {
		t.Fatal("expected error for empty paths")
	}
}

func TestNewRetryInterval_Valid(t *testing.T) {
	ri, err := domain.NewRetryInterval(60 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ri.Duration() != 60*time.Second {
		t.Errorf("expected 60s, got %v", ri.Duration())
	}
}

func TestNewRetryInterval_RejectsZero(t *testing.T) {
	_, err := domain.NewRetryInterval(0)
	if err == nil {
		t.Fatal("expected error for zero RetryInterval")
	}
}

func TestNewRetryInterval_RejectsNegative(t *testing.T) {
	_, err := domain.NewRetryInterval(-1 * time.Second)
	if err == nil {
		t.Fatal("expected error for negative RetryInterval")
	}
}

func TestNewMaxRetries_Valid(t *testing.T) {
	mr, err := domain.NewMaxRetries(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.Int() != 10 {
		t.Errorf("expected 10, got %d", mr.Int())
	}
}

func TestNewMaxRetries_AllowsZero(t *testing.T) {
	mr, err := domain.NewMaxRetries(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.Int() != 0 {
		t.Errorf("expected 0, got %d", mr.Int())
	}
}

func TestNewMaxRetries_RejectsNegative(t *testing.T) {
	_, err := domain.NewMaxRetries(-1)
	if err == nil {
		t.Fatal("expected error for negative MaxRetries")
	}
}
