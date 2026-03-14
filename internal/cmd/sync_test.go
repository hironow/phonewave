package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyncCmd_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	syncCmd := findSubcommand(rootCmd, "sync")

	// then
	if syncCmd == nil {
		t.Fatal("sync subcommand not found")
	}
}

func TestSyncCmd_RejectsArgs(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"sync", "unexpected"})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for extra args on sync")
	}
}

func TestSyncCmd_WithoutInit_FailsWithGuidance(t *testing.T) {
	// given: config does not exist
	dir := t.TempDir()
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", dir + "/nonexistent/config.yaml", "sync"})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when sync called without init")
	}
	output := buf.String() + err.Error()
	if !strings.Contains(strings.ToLower(output), "init") {
		t.Errorf("expected guidance to run init, got: %v", output)
	}
}
