package usecase_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase"
)

type stubInitRunner struct {
	called    bool
	repoPaths []string
	cfgPath   string
	result    *domain.InitResult
	err       error
}

func (s *stubInitRunner) ScanAndInit(_ context.Context, repoPaths []string, cfgPath string) (*domain.InitResult, error) {
	s.called = true
	s.repoPaths = repoPaths
	s.cfgPath = cfgPath
	return s.result, s.err
}

func TestRunInit_ValidCommand(t *testing.T) {
	runner := &stubInitRunner{result: &domain.InitResult{RepoCount: 2}}
	rp1, _ := domain.NewRepoPath("/tmp/repo1")
	rp2, _ := domain.NewRepoPath("/tmp/repo2")
	paths, _ := domain.NewNonEmptyRepoPaths([]domain.RepoPath{rp1, rp2})
	cp, _ := domain.NewConfigPath("/tmp/.phonewave/config.yaml")
	cmd := domain.NewInitCommand(paths, cp)

	result, err := usecase.RunInit(context.Background(), cmd, runner)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.RepoCount != 2 {
		t.Errorf("expected RepoCount 2, got %d", result.RepoCount)
	}
	if !runner.called {
		t.Fatal("expected ScanAndInit to be called")
	}
	if len(runner.repoPaths) != 2 {
		t.Errorf("expected 2 repoPaths, got %d", len(runner.repoPaths))
	}
	if runner.cfgPath != "/tmp/.phonewave/config.yaml" {
		t.Errorf("expected cfgPath /tmp/.phonewave/config.yaml, got %q", runner.cfgPath)
	}
}

func TestRunInit_RunnerError(t *testing.T) {
	runner := &stubInitRunner{err: fmt.Errorf("scan failed")}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	paths, _ := domain.NewNonEmptyRepoPaths([]domain.RepoPath{rp})
	cp, _ := domain.NewConfigPath("/tmp/.phonewave/config.yaml")
	cmd := domain.NewInitCommand(paths, cp)

	_, err := usecase.RunInit(context.Background(), cmd, runner)

	if err == nil {
		t.Fatal("expected error from runner")
	}
}

// Validation tests (empty RepoPaths, empty ConfigPath) are now in
// domain/primitives_test.go — the parse-don't-validate approach ensures
// invalid commands cannot be constructed.
