package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestEnsureStateDir_CreatesGitignore(t *testing.T) {
	// given
	base := t.TempDir()

	// when
	if err := EnsureStateDir(base); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}

	// then: .gitignore exists with explicit entries (not wildcard *)
	gitignorePath := filepath.Join(base, domain.StateDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)
	for _, entry := range []string{"watch.pid", "watch.started", "delivery.log", "events/", ".run/", ".otel.env"} {
		if !strings.Contains(content, entry) {
			t.Errorf(".gitignore should contain %q, got: %q", entry, content)
		}
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

	// then: .gitignore still exists and contains expected entries (no duplication)
	gitignorePath := filepath.Join(base, domain.StateDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)
	if strings.Count(content, "watch.pid") != 1 {
		t.Errorf("watch.pid should appear exactly once, got: %q", content)
	}
}

func TestEnsureStateDir_GitignoreAppendsMissing(t *testing.T) {
	// given: pre-existing gitignore with partial entries
	base := t.TempDir()
	stateDir := filepath.Join(base, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	gitignorePath := filepath.Join(stateDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("watch.pid\nuser-custom-entry\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	if err := EnsureStateDir(base); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}

	// then: missing entries appended, user entry preserved
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "user-custom-entry") {
		t.Error("user-custom-entry should be preserved")
	}
	if !strings.Contains(content, "events/") {
		t.Error("events/ should be appended")
	}
	if strings.Count(content, "watch.pid") != 1 {
		t.Errorf("watch.pid should appear once, got: %q", content)
	}
}
