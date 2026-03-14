package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDoctorCmd_RepairFlag_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when — find doctor subcommand
	var doctorCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "doctor" {
			doctorCmd = sub
			break
		}
	}
	if doctorCmd == nil {
		t.Fatal("doctor subcommand not found")
	}

	// then
	f := doctorCmd.Flags().Lookup("repair")
	if f == nil {
		t.Fatal("--repair flag not found on doctor")
	}
	if f.DefValue != "false" {
		t.Errorf("--repair default = %q, want %q", f.DefValue, "false")
	}
}

func TestDoctorCmd_RunsWithoutError(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"doctor"})

	// when
	err := rootCmd.Execute()

	// then — should succeed, report failed checks, missing config, or ecosystem issues
	if err != nil &&
		!strings.Contains(err.Error(), "check(s) failed") &&
		!strings.Contains(err.Error(), "load config") &&
		!strings.Contains(err.Error(), "ecosystem has issues") {
		t.Fatalf("unexpected error: %v", err)
	}
}
