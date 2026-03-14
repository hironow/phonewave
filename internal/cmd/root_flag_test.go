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

func TestNewRootCommand_NoColorFlag(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("no-color")

	// then
	if f == nil {
		t.Fatal("--no-color PersistentFlag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("--no-color default = %q, want %q", f.DefValue, "false")
	}
}

func TestRootCmd_OutputFlagExists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("output")

	// then
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.DefValue != "text" {
		t.Errorf("default = %q, want text", f.DefValue)
	}
	if f.Shorthand != "o" {
		t.Errorf("shorthand = %q, want o", f.Shorthand)
	}
}

func TestRootCmd_VerboseFlagExists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	f := rootCmd.PersistentFlags().Lookup("verbose")

	// then
	if f == nil {
		t.Fatal("--verbose flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("default = %q, want false", f.DefValue)
	}
	if f.Shorthand != "v" {
		t.Errorf("shorthand = %q, want v", f.Shorthand)
	}
}

func TestRootCmd_VerboseIncreasesStderrOutput(t *testing.T) {
	// given: initialized project
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)
	os.WriteFile(cfgPath, []byte("repositories: []\nroutes: []\n"), 0o644)

	// when: run status without verbose
	root1 := NewRootCommand()
	var stdout1, stderr1 bytes.Buffer
	root1.SetOut(&stdout1)
	root1.SetErr(&stderr1)
	root1.SetArgs([]string{"status", dir})
	root1.Execute()

	// when: run status WITH verbose
	root2 := NewRootCommand()
	var stdout2, stderr2 bytes.Buffer
	root2.SetOut(&stdout2)
	root2.SetErr(&stderr2)
	root2.SetArgs([]string{"-v", "status", dir})
	root2.Execute()

	// then: verbose should produce at least as much stderr as non-verbose
	// (in practice verbose adds DBUG lines)
	if stderr2.Len() < stderr1.Len() {
		t.Errorf("verbose stderr (%d bytes) should be >= non-verbose stderr (%d bytes)",
			stderr2.Len(), stderr1.Len())
	}
}

func TestRootCmd_NoColorSetsEnv(t *testing.T) {
	// given
	origVal := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	t.Cleanup(func() {
		if origVal != "" {
			os.Setenv("NO_COLOR", origVal)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	})

	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)
	os.WriteFile(cfgPath, []byte("repositories: []\nroutes: []\n"), 0o644)

	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--no-color", "status", dir})

	// when
	root.Execute()

	// then: NO_COLOR env should be set
	if got := os.Getenv("NO_COLOR"); got == "" {
		t.Error("expected NO_COLOR env to be set after --no-color flag")
	}

	// also verify no ANSI escape codes in output
	output := buf.String()
	if strings.Contains(output, "\033[") {
		t.Errorf("--no-color output should not contain ANSI codes, got: %q", output)
	}
}
