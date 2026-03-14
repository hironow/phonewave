package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

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
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.StateDir, domain.ConfigFile), "archive-prune"})

	// when
	err := rootCmd.Execute()

	// then — dry-run should not delete the file
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); errors.Is(statErr, fs.ErrNotExist) {
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
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.StateDir, domain.ConfigFile), "archive-prune", "--execute", "--yes"})

	// when
	err := rootCmd.Execute()

	// then — expired file deleted, recent file kept
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); !errors.Is(statErr, fs.ErrNotExist) {
		t.Error("--execute should delete the expired file")
	}
	if _, statErr := os.Stat(recentFile); errors.Is(statErr, fs.ErrNotExist) {
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
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.StateDir, domain.ConfigFile), "--output", "json", "archive-prune"})

	// when
	err := rootCmd.Execute()

	// then — JSON to stdout, file not deleted (dry-run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); errors.Is(statErr, fs.ErrNotExist) {
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
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.StateDir, domain.ConfigFile), "--output", "json", "archive-prune", "--execute"})

	// when
	err := rootCmd.Execute()

	// then — JSON to stdout, file deleted
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); !errors.Is(statErr, fs.ErrNotExist) {
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

func TestArchivePruneCmd_RebuildIndexFlag_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when — find archive-prune subcommand
	var apCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "archive-prune" {
			apCmd = sub
			break
		}
	}
	if apCmd == nil {
		t.Fatal("archive-prune subcommand not found")
	}

	// then
	f := apCmd.Flags().Lookup("rebuild-index")
	if f == nil {
		t.Fatal("--rebuild-index flag not found on archive-prune")
	}
}

func TestArchivePruneCmd_RebuildIndex_ConflictsWithExecute(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", filepath.Join(dir, domain.StateDir, domain.ConfigFile), "archive-prune", "--rebuild-index", "--execute"})

	// when
	err := rootCmd.Execute()

	// then — should fail with conflict error
	if err == nil {
		t.Fatal("expected error when combining --rebuild-index with --execute")
	}
	if !strings.Contains(err.Error(), "rebuild-index") {
		t.Errorf("error should mention rebuild-index, got: %v", err)
	}
}

func TestArchivePruneCmd_RebuildIndex_CreatesIndex(t *testing.T) {
	// given — state directory with archive subdirectory
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	archiveDir := filepath.Join(stateDir, "archive")
	os.MkdirAll(archiveDir, 0o755)
	os.WriteFile(filepath.Join(archiveDir, "2025-01-01.jsonl"), []byte(`{"id":"1","tool":"phonewave"}`+"\n"), 0o644)

	rootCmd := NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--config", filepath.Join(stateDir, domain.ConfigFile), "archive-prune", "--rebuild-index"})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("--rebuild-index failed: %v", err)
	}
	indexPath := filepath.Join(archiveDir, "index.jsonl")
	if _, statErr := os.Stat(indexPath); os.IsNotExist(statErr) {
		t.Error("expected index.jsonl to be created by --rebuild-index")
	}
}

func TestArchivePruneCmd_DaysFlag(t *testing.T) {
	// given — use custom days value
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	// Create a file 10 days old
	oldFile := filepath.Join(eventsDir, "2025-01-01.jsonl")
	os.WriteFile(oldFile, []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	// --days 5 should find the 10-day-old file as a candidate
	rootCmd.SetArgs([]string{"--config", filepath.Join(stateDir, domain.ConfigFile), "archive-prune", "--days", "5"})

	// when
	err := rootCmd.Execute()

	// then — dry-run with --days 5 should report the candidate
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
