package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestCollectCleanTargets_IncludesRunDir(t *testing.T) {
	// given — a state dir with .run/ directory (SQLite stores)
	stateDir := t.TempDir()
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create .run: %v", err)
	}
	// Create a dummy DB inside .run to simulate real state
	if err := os.WriteFile(filepath.Join(runDir, "error_queue.db"), []byte("x"), 0o644); err != nil {
		t.Fatalf("create error_queue.db: %v", err)
	}

	// when
	targets := collectCleanTargets(stateDir)

	// then — .run should be in the target list
	found := false
	for _, t := range targets {
		if filepath.Base(t) == ".run" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("collectCleanTargets should include .run directory, got: %v", targets)
	}
}

func TestCollectCleanTargets_DoesNotIncludeErrorsDB(t *testing.T) {
	// given — a state dir with no errors.db (legacy file should not be a candidate)
	stateDir := t.TempDir()

	// when
	targets := collectCleanTargets(stateDir)

	// then — errors.db should NOT be in the candidates list
	for _, target := range targets {
		if filepath.Base(target) == "errors.db" {
			t.Errorf("collectCleanTargets should not include legacy errors.db, got: %v", targets)
		}
	}
}

func TestCollectCleanTargets_AllExpectedCandidates(t *testing.T) {
	// given — a state dir with all expected runtime state
	stateDir := t.TempDir()
	for _, name := range []string{"delivery.log", "watch.pid", "watch.started"} {
		if err := os.WriteFile(filepath.Join(stateDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	for _, dir := range []string{"events", ".run"} {
		if err := os.MkdirAll(filepath.Join(stateDir, dir), 0o755); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}

	// when
	targets := collectCleanTargets(stateDir)

	// then — all 5 expected candidates should be present
	expected := map[string]bool{
		"delivery.log":  false,
		".run":          false,
		"watch.pid":     false,
		"watch.started": false,
		"events":        false,
	}
	for _, target := range targets {
		name := filepath.Base(target)
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected %s in clean targets, got: %v", name, targets)
		}
	}
}

func TestCleanCmd_IncludesSkillsRefVenv(t *testing.T) {
	// given — state dir + skills-ref venv exists in temp
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	runDir := filepath.Join(stateDir, ".run")
	os.MkdirAll(runDir, 0755)

	// Create a fake venv at the expected location
	venvDir := filepath.Join(os.TempDir(), domain.SkillsRefVenvName)
	os.MkdirAll(venvDir, 0755)
	defer os.RemoveAll(venvDir) // cleanup

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetIn(strings.NewReader("n\n")) // decline deletion
	cmd.SetArgs([]string{"clean", dir})

	// when
	cmd.Execute()

	// then — output should mention the venv path
	output := buf.String()
	if !strings.Contains(output, "phonewave-skills-ref-venv") {
		t.Errorf("expected skills-ref venv in clean targets, got:\n%s", output)
	}
}
