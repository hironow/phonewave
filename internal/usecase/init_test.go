package usecase_test

import (
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

func (s *stubInitRunner) ScanAndInit(repoPaths []string, cfgPath string) (*domain.InitResult, error) {
	s.called = true
	s.repoPaths = repoPaths
	s.cfgPath = cfgPath
	return s.result, s.err
}

func TestRunInit_ValidCommand(t *testing.T) {
	runner := &stubInitRunner{result: &domain.InitResult{RepoCount: 2}}
	cmd := domain.InitCommand{
		RepoPaths:  []string{"/tmp/repo1", "/tmp/repo2"},
		ConfigPath: "/tmp/phonewave.yaml",
	}

	result, err := usecase.RunInit(cmd, runner)

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
	if runner.cfgPath != "/tmp/phonewave.yaml" {
		t.Errorf("expected cfgPath /tmp/phonewave.yaml, got %q", runner.cfgPath)
	}
}

func TestRunInit_EmptyRepoPaths(t *testing.T) {
	runner := &stubInitRunner{}
	cmd := domain.InitCommand{RepoPaths: nil, ConfigPath: "/tmp/phonewave.yaml"}

	_, err := usecase.RunInit(cmd, runner)

	if err == nil {
		t.Fatal("expected error for empty RepoPaths")
	}
	if runner.called {
		t.Fatal("expected ScanAndInit not to be called")
	}
}

func TestRunInit_EmptyConfigPath(t *testing.T) {
	runner := &stubInitRunner{}
	cmd := domain.InitCommand{RepoPaths: []string{"/tmp/repo"}, ConfigPath: ""}

	_, err := usecase.RunInit(cmd, runner)

	if err == nil {
		t.Fatal("expected error for empty ConfigPath")
	}
	if runner.called {
		t.Fatal("expected ScanAndInit not to be called")
	}
}

func TestRunInit_RunnerError(t *testing.T) {
	runner := &stubInitRunner{err: fmt.Errorf("scan failed")}
	cmd := domain.InitCommand{
		RepoPaths:  []string{"/tmp/repo"},
		ConfigPath: "/tmp/phonewave.yaml",
	}

	_, err := usecase.RunInit(cmd, runner)

	if err == nil {
		t.Fatal("expected error from runner")
	}
}
