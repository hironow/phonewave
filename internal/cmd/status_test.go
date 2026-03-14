package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestStatusCmd_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	statusCmd := findSubcommand(rootCmd, "status")

	// then
	if statusCmd == nil {
		t.Fatal("status subcommand not found")
	}
}

func TestStatusCmd_JSONOutput(t *testing.T) {
	// given: manually create a minimal config so status can load it
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)
	os.WriteFile(cfgPath, []byte("repositories: []\nroutes: []\n"), 0o644)

	rootCmd := NewRootCommand()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"status", "-o", "json", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("status -o json failed: %v", err)
	}
	if !json.Valid(stdout.Bytes()) {
		t.Errorf("stdout is not valid JSON: %s", stdout.String())
	}
}

func TestStatusCmd_WithoutInit_FailsWithGuidance(t *testing.T) {
	// given: config does not exist
	dir := t.TempDir()
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", dir + "/nonexistent/config.yaml", "status", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when status called without init")
	}
	output := buf.String() + err.Error()
	if !strings.Contains(strings.ToLower(output), "init") {
		t.Errorf("expected guidance to run init, got: %v", output)
	}
}
