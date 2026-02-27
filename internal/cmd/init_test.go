package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave"
)

func TestInitCmd_AlreadyInitialized(t *testing.T) {
	// given: config file already exists
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, phonewave.ConfigFile)
	if err := os.WriteFile(cfgPath, []byte("repositories: []\n"), 0644); err != nil {
		t.Fatalf("create config: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", cfgPath, "init", dir})

	// when
	err := cmd.Execute()

	// then: should fail with "already exists" message
	if err == nil {
		t.Fatal("expected error for already initialized, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "already exists") && !strings.Contains(got, "already initialized") {
		t.Errorf("expected 'already exists' or 'already initialized' in error, got: %s", got)
	}
}
