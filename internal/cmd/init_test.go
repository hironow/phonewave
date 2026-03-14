package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/phonewave/internal/domain"
)

func TestInitCmd_ForceFlag_OverwritesExisting(t *testing.T) {
	// given: .phonewave/ directory already exists with config
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)
	os.WriteFile(cfgPath, []byte("old: config\n"), 0o644)

	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"--config", cfgPath, "init", "--force", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init --force failed: %v", err)
	}
}

func TestInitCmd_WithoutForce_FailsIfExists(t *testing.T) {
	// given: already initialized
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)
	os.WriteFile(cfgPath, []byte("old: config\n"), 0o644)

	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"--config", cfgPath, "init", dir})

	// when
	err := rootCmd.Execute()

	// then — should fail and suggest --force
	if err == nil {
		t.Fatal("expected error when already initialized without --force")
	}
	if !strings.Contains(err.Error(), "force") {
		t.Errorf("error should mention --force, got: %v", err)
	}
}

func TestInitCmd_OtelBackend_CreatesOtelEnv(t *testing.T) {
	// given: a temp dir as repo path with config pointing inside it
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)

	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"--config", cfgPath, "init", "--otel-backend", "jaeger", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init --otel-backend jaeger failed: %v", err)
	}
	otelPath := filepath.Join(stateDir, ".otel.env")
	data, readErr := os.ReadFile(otelPath)
	if readErr != nil {
		t.Fatalf(".otel.env not created: %v", readErr)
	}
	if !strings.Contains(string(data), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Errorf("expected OTEL_EXPORTER_OTLP_ENDPOINT in .otel.env, got:\n%s", data)
	}
}

func TestInitCmd_OtelFlags_Exist(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when — find init subcommand
	var initCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "init" {
			initCmd = sub
			break
		}
	}
	if initCmd == nil {
		t.Fatal("init subcommand not found")
	}

	// then — otel flags exist
	for _, flag := range []string{"otel-backend", "otel-entity", "otel-project"} {
		if initCmd.Flags().Lookup(flag) == nil {
			t.Errorf("init flag --%s not found", flag)
		}
	}
}
