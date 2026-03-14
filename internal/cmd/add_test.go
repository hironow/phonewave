package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddCmd_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	addCmd := findSubcommand(rootCmd, "add")

	// then
	if addCmd == nil {
		t.Fatal("add subcommand not found")
	}
}

func TestAddCmd_RequiresExactlyOneArg(t *testing.T) {
	// given: no args
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"add"})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when add called without args")
	}
}

func TestAddCmd_WithoutInit_FailsWithGuidance(t *testing.T) {
	// given: config does not exist
	dir := t.TempDir()
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", dir + "/nonexistent/config.yaml", "add", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when add called without init")
	}
	output := buf.String() + err.Error()
	if !strings.Contains(strings.ToLower(output), "init") {
		t.Errorf("expected guidance to run init, got: %v", output)
	}
}
