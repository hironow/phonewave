package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
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

func TestInitCmd_Snapshot(t *testing.T) {
	// given — init with an empty repo dir (no SKILL.md → empty routing table)
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)

	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"--config", cfgPath, "init", "--force", dir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// then — walk state dir and verify exact file list
	got := walkStateDir(t, stateDir)

	want := []string{
		".gitignore",
		".run/",
		".run/resolved.yaml",
		"config.yaml",
		"events/",
		"insights/",
	}

	if !slices.Equal(want, got) {
		t.Errorf("init snapshot mismatch\nwant: %v\ngot:  %v", want, got)
	}
}

func TestInitCmd_ConfigHeader(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)

	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"--config", cfgPath, "init", "--force", dir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.HasPrefix(string(data), "# phonewave configuration") {
		t.Errorf("expected config header comment, got:\n%s", string(data)[:min(len(data), 80)])
	}
}

func TestInitCmd_GitignoreComplete(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	cfgPath := filepath.Join(stateDir, domain.ConfigFile)

	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"--config", cfgPath, "init", "--force", dir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(stateDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)
	for _, entry := range []string{"watch.pid", "watch.started", "events/", ".run/", ".otel.env", "!config.yaml"} {
		if !strings.Contains(content, entry) {
			t.Errorf("expected %q in .gitignore, got:\n%s", entry, content)
		}
	}
}

func walkStateDir(t *testing.T, stateDir string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(stateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(stateDir, path)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			rel += "/"
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk state dir: %v", err)
	}
	sort.Strings(paths)
	return paths
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
