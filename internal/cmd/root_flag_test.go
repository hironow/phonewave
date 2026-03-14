package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import "testing"

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
