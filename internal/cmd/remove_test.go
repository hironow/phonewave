package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"strings"
	"testing"
)

func TestRemoveCmd_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	removeCmd := findSubcommand(rootCmd, "remove")

	// then
	if removeCmd == nil {
		t.Fatal("remove subcommand not found")
	}
}

func TestRemoveCmd_RequiresExactlyOneArg(t *testing.T) {
	// given: no args
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"remove"})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when remove called without args")
	}
}

func TestRemoveCmd_WithoutInit_FailsWithGuidance(t *testing.T) {
	// given: config does not exist
	dir := t.TempDir()
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", dir + "/nonexistent/config.yaml", "remove", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when remove called without init")
	}
	output := buf.String() + err.Error()
	if !strings.Contains(strings.ToLower(output), "init") {
		t.Errorf("expected guidance to run init, got: %v", output)
	}
}
