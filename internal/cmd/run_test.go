package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func findSubcommand(rootCmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

func TestRunCmd_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	runCmd := findSubcommand(rootCmd, "run")

	// then
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}
}

func TestRunCmd_DryRunFlag(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	runCmd := findSubcommand(rootCmd, "run")
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	// when
	f := runCmd.Flags().Lookup("dry-run")

	// then
	if f == nil {
		t.Fatal("--dry-run flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("--dry-run default = %q, want %q", f.DefValue, "false")
	}
	if f.Shorthand != "n" {
		t.Errorf("--dry-run shorthand = %q, want %q", f.Shorthand, "n")
	}
}

func TestRunCmd_MaxRetriesFlag(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	runCmd := findSubcommand(rootCmd, "run")
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	// when
	f := runCmd.Flags().Lookup("max-retries")

	// then
	if f == nil {
		t.Fatal("--max-retries flag not found")
	}
	if f.DefValue != "10" {
		t.Errorf("--max-retries default = %q, want %q", f.DefValue, "10")
	}
	if f.Shorthand != "m" {
		t.Errorf("--max-retries shorthand = %q, want %q", f.Shorthand, "m")
	}
}

func TestRunCmd_RetryIntervalFlag(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	runCmd := findSubcommand(rootCmd, "run")
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	// when
	f := runCmd.Flags().Lookup("retry-interval")

	// then
	if f == nil {
		t.Fatal("--retry-interval flag not found")
	}
	if f.DefValue != "1m0s" {
		t.Errorf("--retry-interval default = %q, want %q", f.DefValue, "1m0s")
	}
	if f.Shorthand != "r" {
		t.Errorf("--retry-interval shorthand = %q, want %q", f.Shorthand, "r")
	}
}

func TestRunCmd_WithoutInit_FailsWithGuidance(t *testing.T) {
	// given: no init performed — config file does not exist
	dir := t.TempDir()
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", dir + "/nonexistent/config.yaml", "run"})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when running without init")
	}
	output := buf.String() + err.Error()
	if !strings.Contains(strings.ToLower(output), "init") {
		t.Errorf("expected guidance to run init, got: %v", output)
	}
}
