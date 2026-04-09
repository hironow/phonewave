package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
	_ "modernc.org/sqlite"
)

// seedDeadLetters creates a delivery store with N dead-lettered items
// (retry_count >= maxDeliveryRetryCount which is 3).
func seedDeadLetters(t *testing.T, stateDir string, n int) {
	t.Helper()

	store, err := session.NewSQLiteDeliveryStore(stateDir)
	if err != nil {
		t.Fatalf("create delivery store: %v", err)
	}
	// Stage N items
	for i := range n {
		path := filepath.Join("outbox", "test-"+string(rune('a'+i))+".yaml")
		if err := store.StageDelivery(t.Context(), path, []byte("data"), []string{"target"}); err != nil {
			t.Fatalf("stage delivery %d: %v", i, err)
		}
	}
	store.Close()

	// Bump retry_count to 3 (dead-letter threshold) via raw SQL
	dbPath := filepath.Join(stateDir, ".run", "delivery.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE staged_delivery SET retry_count = 3`); err != nil {
		t.Fatalf("bump retry_count: %v", err)
	}
}

func TestDeadLettersPurge_DryRunDefault(t *testing.T) {
	// given — 2 dead-lettered items
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)
	seedDeadLetters(t, stateDir, 2)

	rootCmd := NewRootCommand()
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{
		"--config", filepath.Join(stateDir, domain.ConfigFile),
		"dead-letters", "purge",
	})

	// when
	err := rootCmd.Execute()

	// then — dry-run shows count, does not delete
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := errBuf.String()
	if !strings.Contains(output, "2 dead-lettered item(s) would be purged") {
		t.Errorf("expected dry-run message with count, got: %q", output)
	}
	if strings.Contains(output, "Purged") {
		t.Error("dry-run should NOT purge")
	}
}

func TestDeadLettersPurge_ExecuteDeletesItems(t *testing.T) {
	// given — 2 dead-lettered items
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)
	seedDeadLetters(t, stateDir, 2)

	rootCmd := NewRootCommand()
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{
		"--config", filepath.Join(stateDir, domain.ConfigFile),
		"dead-letters", "purge", "--execute", "--yes",
	})

	// when
	err := rootCmd.Execute()

	// then — items purged
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := errBuf.String()
	if !strings.Contains(output, "Purged 2 dead-lettered item(s)") {
		t.Errorf("expected purge confirmation, got: %q", output)
	}

	// verify items are actually gone
	store, storeErr := session.NewSQLiteDeliveryStore(stateDir)
	if storeErr != nil {
		t.Fatalf("reopen store: %v", storeErr)
	}
	defer store.Close()
	remaining, countErr := store.DeadLetterCount(t.Context())
	if countErr != nil {
		t.Fatalf("count after purge: %v", countErr)
	}
	if remaining != 0 {
		t.Errorf("expected 0 dead letters after purge, got %d", remaining)
	}
}

func TestDeadLettersPurge_NoDatabaseShowsMessage(t *testing.T) {
	// given — no delivery database exists
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)

	rootCmd := NewRootCommand()
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{
		"--config", filepath.Join(stateDir, domain.ConfigFile),
		"dead-letters", "purge",
	})

	// when
	err := rootCmd.Execute()

	// then — no error, informative message
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := errBuf.String()
	if !strings.Contains(output, "No delivery database") {
		t.Errorf("expected 'No delivery database' message, got: %q", output)
	}
}

func TestDeadLettersPurge_ZeroDeadLetters(t *testing.T) {
	// given — delivery store exists but no dead letters
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0o755)

	// Create store (creates DB with schema) but don't seed any dead letters
	store, err := session.NewSQLiteDeliveryStore(stateDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.Close()

	rootCmd := NewRootCommand()
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{
		"--config", filepath.Join(stateDir, domain.ConfigFile),
		"dead-letters", "purge",
	})

	// when
	err = rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := errBuf.String()
	if !strings.Contains(output, "No dead-lettered items") {
		t.Errorf("expected 'No dead-lettered items' message, got: %q", output)
	}
}
