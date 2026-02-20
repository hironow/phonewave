package main

import (
	"bytes"
	"testing"

	cmd "github.com/hironow/phonewave/internal/cmd"
	"github.com/spf13/cobra"
)

func TestRootCommand_Help(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected help output, got empty string")
	}
}

func TestRootCommand_UnknownSubcommand(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestSubcommands_Exist(t *testing.T) {
	rootCmd := cmd.NewRootCommand()

	expected := []string{"init", "add", "remove", "sync", "doctor", "run", "status"}
	for _, name := range expected {
		found := false
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestRunCommand_HasFlags(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	var runCmd *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "run" {
			runCmd = c
			break
		}
	}
	if runCmd == nil {
		t.Fatal("run subcommand not found")
	}

	flags := []string{"dry-run", "retry-interval", "max-retries"}
	for _, name := range flags {
		if runCmd.Flags().Lookup(name) == nil {
			t.Errorf("run subcommand missing flag %q", name)
		}
	}
}

func TestRootCommand_PersistentFlags(t *testing.T) {
	rootCmd := cmd.NewRootCommand()

	if rootCmd.PersistentFlags().Lookup("verbose") == nil {
		t.Error("root command missing persistent flag 'verbose'")
	}
}
