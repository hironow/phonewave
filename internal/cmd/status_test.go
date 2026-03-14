package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"strings"
	"testing"
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
