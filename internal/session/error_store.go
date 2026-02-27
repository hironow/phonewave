package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	phonewave "github.com/hironow/phonewave"

	_ "modernc.org/sqlite"
)

// Compile-time check that SQLiteErrorStore implements phonewave.ErrorStore.
var _ phonewave.ErrorStore = (*SQLiteErrorStore)(nil)

// SQLiteErrorStore implements ErrorStore using a SQLite database as the
// durable error queue. Failed D-Mail deliveries are recorded with their
// payload and metadata, enabling retry without filesystem sidecar files.
type SQLiteErrorStore struct {
	db *sql.DB
}

// NewSQLiteErrorStore opens (or creates) a SQLite database at
// stateDir/.run/errors.db and initialises the schema.
func NewSQLiteErrorStore(stateDir string) (*SQLiteErrorStore, error) {
	dbPath := filepath.Join(stateDir, ".run", "errors.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("error store: create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error store: open db: %w", err)
	}
	// SQLite is a single-file database: limit to one connection to prevent
	// "database is locked" errors from the Go connection pool. WAL mode
	// handles concurrent access from OTHER processes; this setting governs
	// connections within THIS process.
	db.SetMaxOpenConns(1)
	// Set PRAGMAs explicitly — modernc.org/sqlite does not support
	// underscore-prefixed query parameters like mattn/go-sqlite3.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("error store: %s: %w", pragma, err)
		}
	}
	if err := createErrorSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteErrorStore{db: db}, nil
}

func createErrorSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS delivery_errors (
		name          TEXT PRIMARY KEY,
		source_outbox TEXT    NOT NULL,
		kind          TEXT    NOT NULL,
		original_name TEXT    NOT NULL,
		data          BLOB   NOT NULL,
		attempts      INTEGER NOT NULL DEFAULT 1,
		error         TEXT    NOT NULL,
		created_at    TEXT    NOT NULL,
		updated_at    TEXT    NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("error store: create schema: %w", err)
	}
	return nil
}

// RecordFailure inserts a failed D-Mail into the error table. Idempotent:
// re-recording the same name is silently ignored (INSERT OR IGNORE).
// Uses BEGIN IMMEDIATE to prevent concurrent access deadlocks.
func (s *SQLiteErrorStore) RecordFailure(entry phonewave.RetryEntry) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("error store: get conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("error store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	_, err = conn.ExecContext(ctx,
		`INSERT OR IGNORE INTO delivery_errors
			(name, source_outbox, kind, original_name, data, attempts, error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Name,
		entry.SourceOutbox,
		entry.Kind,
		entry.OriginalName,
		entry.Data,
		entry.Attempts,
		entry.Error,
		entry.CreatedAt.Format(time.RFC3339),
		entry.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("error store: record %s: %w", entry.Name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error store: commit: %w", err)
	}
	committed = true
	return nil
}

// ListPending returns entries with attempts < maxRetries, ordered by
// created_at ascending (oldest first).
func (s *SQLiteErrorStore) ListPending(maxRetries int) ([]phonewave.RetryEntry, error) {
	rows, err := s.db.Query(
		`SELECT name, source_outbox, kind, original_name, data, attempts, error, created_at, updated_at
		FROM delivery_errors
		WHERE attempts < ?
		ORDER BY created_at ASC`,
		maxRetries,
	)
	if err != nil {
		return nil, fmt.Errorf("error store: list pending: %w", err)
	}
	defer rows.Close()

	var entries []phonewave.RetryEntry
	for rows.Next() {
		var e phonewave.RetryEntry
		var createdStr, updatedStr string
		if err := rows.Scan(
			&e.Name, &e.SourceOutbox, &e.Kind, &e.OriginalName,
			&e.Data, &e.Attempts, &e.Error, &createdStr, &updatedStr,
		); err != nil {
			return nil, fmt.Errorf("error store: scan row: %w", err)
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error store: rows iter: %w", err)
	}
	return entries, nil
}

// MarkRetried increments the attempt counter and updates the error message.
// Uses BEGIN IMMEDIATE to prevent concurrent access deadlocks.
func (s *SQLiteErrorStore) MarkRetried(name string, newError string) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("error store: get conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("error store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := conn.ExecContext(ctx,
		`UPDATE delivery_errors SET attempts = attempts + 1, error = ?, updated_at = ? WHERE name = ?`,
		newError, now, name,
	)
	if err != nil {
		return fmt.Errorf("error store: mark retried %s: %w", name, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("error store: entry %q not found", name)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error store: commit: %w", err)
	}
	committed = true
	return nil
}

// RemoveEntry deletes an entry from the error table.
func (s *SQLiteErrorStore) RemoveEntry(name string) error {
	result, err := s.db.Exec(`DELETE FROM delivery_errors WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("error store: remove %s: %w", name, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("error store: entry %q not found", name)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteErrorStore) Close() error {
	return s.db.Close()
}
