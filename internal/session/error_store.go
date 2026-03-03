package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	_ "modernc.org/sqlite"
)

// SQLiteErrorStore manages failed D-Mail delivery records in a SQLite database.
// All write operations use BEGIN IMMEDIATE to prevent concurrent write conflicts.
type SQLiteErrorStore struct {
	db *sql.DB
}

// ErrorEntry holds a single error queue record.
type ErrorEntry struct {
	Name         string
	Data         []byte
	SourceOutbox string
	Kind         string
	ErrorMessage string
	RetryCount   int
}

// NewSQLiteErrorStore opens (or creates) a SQLite error store at
// {stateDir}/errors.db and initialises the schema.
func NewSQLiteErrorStore(stateDir string) (*SQLiteErrorStore, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("error store: create dir: %w", err)
	}

	dbPath := filepath.Join(stateDir, "errors.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error store: open db: %w", err)
	}

	db.SetMaxOpenConns(1)

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
	if err := os.Chmod(dbPath, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("error store: chmod db: %w", err)
	}

	return &SQLiteErrorStore{db: db}, nil
}

func createErrorSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS errors (
		name          TEXT PRIMARY KEY,
		data          BLOB    NOT NULL,
		source_outbox TEXT    NOT NULL,
		kind          TEXT    NOT NULL,
		error_message TEXT    NOT NULL DEFAULT '',
		retry_count   INTEGER NOT NULL DEFAULT 0,
		resolved      INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return fmt.Errorf("error store: create schema: %w", err)
	}
	return nil
}

// RecordError inserts a failed D-Mail into the error store.
// Idempotent: re-recording the same name is silently ignored (INSERT OR IGNORE).
// Uses BEGIN IMMEDIATE for write safety under concurrent access.
func (s *SQLiteErrorStore) RecordError(name string, data []byte, meta domain.ErrorMetadata) error {
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
		`INSERT OR IGNORE INTO errors (name, data, source_outbox, kind, error_message)
		 VALUES (?, ?, ?, ?, ?)`,
		name, data, meta.SourceOutbox, meta.Kind, meta.Error,
	)
	if err != nil {
		return fmt.Errorf("error store: record %s: %w", name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error store: commit record %s: %w", name, err)
	}
	committed = true
	return nil
}

// IncrementRetry increments the retry_count and updates the error message.
// Uses BEGIN IMMEDIATE for write safety under concurrent access.
func (s *SQLiteErrorStore) IncrementRetry(name string, newError string) error {
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
		`UPDATE errors SET retry_count = retry_count + 1, error_message = ? WHERE name = ?`,
		newError, name,
	)
	if err != nil {
		return fmt.Errorf("error store: increment retry %s: %w", name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error store: commit increment %s: %w", name, err)
	}
	committed = true
	return nil
}

// PendingErrors returns all unresolved error entries with retry_count below maxRetries.
func (s *SQLiteErrorStore) PendingErrors(maxRetries int) ([]ErrorEntry, error) {
	rows, err := s.db.Query(
		`SELECT name, data, source_outbox, kind, error_message, retry_count
		 FROM errors WHERE resolved = 0 AND retry_count < ?`, maxRetries,
	)
	if err != nil {
		return nil, fmt.Errorf("error store: query pending: %w", err)
	}
	defer rows.Close()

	var entries []ErrorEntry
	for rows.Next() {
		var e ErrorEntry
		if err := rows.Scan(&e.Name, &e.Data, &e.SourceOutbox, &e.Kind, &e.ErrorMessage, &e.RetryCount); err != nil {
			return nil, fmt.Errorf("error store: scan row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error store: rows iter: %w", err)
	}
	return entries, nil
}

// MarkResolved marks an error entry as resolved (successfully retried).
// Uses BEGIN IMMEDIATE for write safety under concurrent access.
func (s *SQLiteErrorStore) MarkResolved(name string) error {
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

	_, err = conn.ExecContext(ctx, `UPDATE errors SET resolved = 1 WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("error store: mark resolved %s: %w", name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error store: commit resolved %s: %w", name, err)
	}
	committed = true
	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteErrorStore) Close() error {
	return s.db.Close()
}
