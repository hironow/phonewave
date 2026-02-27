package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave"
)

func TestCleanCmd_NothingToClean(t *testing.T) {
	// given: empty directory with no state
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, phonewave.ConfigFile)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", cfgPath, "clean", "--yes"})

	// when
	err := cmd.Execute()

	// then: should succeed with "nothing to clean" message
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "Nothing to clean") {
		t.Errorf("expected 'Nothing to clean' in output, got: %s", got)
	}
}

func TestCleanCmd_DeletesStateDir(t *testing.T) {
	// given: initialized state (config file + state dir)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, phonewave.ConfigFile)
	if err := os.WriteFile(cfgPath, []byte("repositories: []\n"), 0644); err != nil {
		t.Fatalf("create config: %v", err)
	}
	stateDir := filepath.Join(dir, phonewave.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", cfgPath, "clean", "--yes"})

	// when
	err := cmd.Execute()

	// then: should succeed and delete state dir + config
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Error("expected state dir to be deleted")
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("expected config file to be deleted")
	}
}
