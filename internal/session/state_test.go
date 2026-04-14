package session_test

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestEnsureStateDir_CreatesGitignore(t *testing.T) {
	// given
	base := t.TempDir()

	// when
	if err := session.EnsurePhonewaveStateDir(base); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}

	// then: .gitignore exists with explicit entries (not wildcard *)
	gitignorePath := filepath.Join(base, domain.StateDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)
	for _, entry := range []string{"watch.pid", "watch.started", "provider-state.json", "delivery.log", "events/", ".run/", ".otel.env", "!config.yaml"} {
		if !strings.Contains(content, entry) {
			t.Errorf(".gitignore should contain %q, got: %q", entry, content)
		}
	}
}

func TestEnsurePhonewaveStateDir_Snapshot(t *testing.T) {
	base := t.TempDir()

	if err := session.EnsurePhonewaveStateDir(base); err != nil {
		t.Fatalf("EnsurePhonewaveStateDir: %v", err)
	}

	stateDir := filepath.Join(base, domain.StateDir)
	got := walkStateDir(t, stateDir)

	want := []string{
		".gitignore",
		".run/",
		"events/",
		"insights/",
	}

	if !slices.Equal(want, got) {
		t.Errorf("state dir snapshot mismatch\nwant: %v\ngot:  %v", want, got)
	}
}

func walkStateDir(t *testing.T, stateDir string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(stateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(stateDir, path)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			rel += "/"
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk state dir: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func TestEnsureStateDir_GitignoreIdempotent(t *testing.T) {
	// given
	base := t.TempDir()

	// when: call twice
	if err := session.EnsurePhonewaveStateDir(base); err != nil {
		t.Fatalf("first EnsureStateDir: %v", err)
	}
	if err := session.EnsurePhonewaveStateDir(base); err != nil {
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
	if err := session.EnsurePhonewaveStateDir(base); err != nil {
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
