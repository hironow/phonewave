package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestArchivePruneCmd_DryRunDefault(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "2025-01-01.jsonl")
	os.WriteFile(oldFile, []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.ConfigFile), "archive-prune"})

	// when
	err := rootCmd.Execute()

	// then — dry-run should not delete the file
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); os.IsNotExist(statErr) {
		t.Error("dry-run should NOT delete the file")
	}
	output := errBuf.String()
	if !strings.Contains(output, "dry-run") {
		t.Errorf("expected dry-run message, got: %q", output)
	}
}

func TestArchivePruneCmd_ExecuteDeletesExpired(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "2025-01-01.jsonl")
	os.WriteFile(oldFile, []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	// recent file should be kept
	recentFile := filepath.Join(eventsDir, "2026-02-28.jsonl")
	os.WriteFile(recentFile, []byte(`{"id":"recent"}`+"\n"), 0o644)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.ConfigFile), "archive-prune", "--execute", "--yes"})

	// when
	err := rootCmd.Execute()

	// then — expired file deleted, recent file kept
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); !os.IsNotExist(statErr) {
		t.Error("--execute should delete the expired file")
	}
	if _, statErr := os.Stat(recentFile); os.IsNotExist(statErr) {
		t.Error("recent file should NOT be deleted")
	}
	output := errBuf.String()
	if !strings.Contains(output, "Pruned") {
		t.Errorf("expected 'Pruned' message, got: %q", output)
	}
}

func TestArchivePruneCmd_JSONOutput_DryRun(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "2025-01-01.jsonl")
	os.WriteFile(oldFile, []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.ConfigFile), "--output", "json", "archive-prune"})

	// when
	err := rootCmd.Execute()

	// then — JSON to stdout, file not deleted (dry-run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); os.IsNotExist(statErr) {
		t.Error("dry-run should NOT delete the file")
	}

	var result struct {
		Candidates int      `json:"candidates"`
		Deleted    int      `json:"deleted"`
		Files      []string `json:"files"`
	}
	if jsonErr := json.Unmarshal(outBuf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", jsonErr, outBuf.String())
	}
	if result.Candidates != 1 {
		t.Errorf("candidates = %d, want 1", result.Candidates)
	}
	if result.Deleted != 0 {
		t.Errorf("deleted = %d, want 0 (dry-run)", result.Deleted)
	}
}

func TestArchivePruneCmd_JSONOutput_Execute(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "2025-01-01.jsonl")
	os.WriteFile(oldFile, []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.ConfigFile), "--output", "json", "archive-prune", "--execute"})

	// when
	err := rootCmd.Execute()

	// then — JSON to stdout, file deleted
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); !os.IsNotExist(statErr) {
		t.Error("--execute should delete the expired file")
	}

	var result struct {
		Candidates int      `json:"candidates"`
		Deleted    int      `json:"deleted"`
		Files      []string `json:"files"`
	}
	if jsonErr := json.Unmarshal(outBuf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", jsonErr, outBuf.String())
	}
	if result.Candidates != 1 {
		t.Errorf("candidates = %d, want 1", result.Candidates)
	}
	if result.Deleted != 1 {
		t.Errorf("deleted = %d, want 1", result.Deleted)
	}
}
