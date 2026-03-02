package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave"
)

func TestEnsureStateDir_CreatesGitignore(t *testing.T) {
	// given
	base := t.TempDir()

	// when
	if err := EnsureStateDir(base); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}

	// then: .gitignore exists in .phonewave/
	gitignorePath := filepath.Join(base, phonewave.StateDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), "*") {
		t.Errorf(".gitignore should contain wildcard '*', got: %q", string(data))
	}
}

func TestEnsureStateDir_GitignoreIdempotent(t *testing.T) {
	// given
	base := t.TempDir()

	// when: call twice
	if err := EnsureStateDir(base); err != nil {
		t.Fatalf("first EnsureStateDir: %v", err)
	}
	if err := EnsureStateDir(base); err != nil {
		t.Fatalf("second EnsureStateDir: %v", err)
	}

	// then: .gitignore still exists and is valid
	gitignorePath := filepath.Join(base, phonewave.StateDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), "*") {
		t.Errorf(".gitignore should contain wildcard '*', got: %q", string(data))
	}
}
