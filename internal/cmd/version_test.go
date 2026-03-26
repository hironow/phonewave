package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCmd_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when
	versionCmd := findSubcommand(rootCmd, "version")

	// then
	if versionCmd == nil {
		t.Fatal("version subcommand not found")
	}
}

func TestVersionCmd_JsonFlag_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	versionCmd := findSubcommand(rootCmd, "version")
	if versionCmd == nil {
		t.Fatal("version subcommand not found")
	}

	// when
	f := versionCmd.Flags().Lookup("json")

	// then
	if f == nil {
		t.Fatal("--json flag not found on version command")
	}
	if f.Shorthand != "j" {
		t.Errorf("--json shorthand = %q, want %q", f.Shorthand, "j")
	}
	if f.DefValue != "false" {
		t.Errorf("--json default = %q, want %q", f.DefValue, "false")
	}
}

func TestVersionCmd_TextOutput(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"version"})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "phonewave") {
		t.Errorf("expected output to contain 'phonewave', got: %q", output)
	}
}

func TestVersionCmd_JsonOutput(t *testing.T) {
	// given
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"version", "--json"})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("version --json failed: %v", err)
	}

	var info map[string]string
	if jsonErr := json.Unmarshal(buf.Bytes(), &info); jsonErr != nil {
		t.Fatalf("failed to parse JSON output: %v (output: %q)", jsonErr, buf.String())
	}

	for _, key := range []string{"version", "commit", "date", "go", "os", "arch"} {
		if _, ok := info[key]; !ok {
			t.Errorf("JSON output missing key %q", key)
		}
	}
}

func TestVersionCmd_RejectsArgs(t *testing.T) {
	// given
	cmd := NewRootCommand()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"version", "unexpected"})

	// when
	err := cmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error for extra args on version")
	}
}
