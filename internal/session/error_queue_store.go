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

// SQLiteErrorQueueStore implements domain.ErrorQueueStore using SQLite
// with atomic claim semantics for concurrent daemon safety.
// All write operations use BEGIN IMMEDIATE to prevent deadlocks.
type SQLiteErrorQueueStore struct {
	db *sql.DB
}

// Compile-time check that SQLiteErrorQueueStore implements ErrorQueueStore.
var _ domain.ErrorQueueStore = (*SQLiteErrorQueueStore)(nil)

// NewSQLiteErrorQueueStore opens (or creates) a SQLite error queue store
// at {stateDir}/error_queue.db and initialises the schema.
func NewSQLiteErrorQueueStore(stateDir string) (*SQLiteErrorQueueStore, error) {
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("error queue store: create dir: %w", err)
	}

	dbPath := filepath.Join(runDir, "error_queue.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error queue store: open db: %w", err)
	}

	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("error queue store: %s: %w", pragma, err)
		}
	}

	if err := createErrorQueueSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteErrorQueueStore{db: db}, nil
}

func createErrorQueueSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS error_queue (
		name          TEXT PRIMARY KEY,
		data          BLOB    NOT NULL,
		source_outbox TEXT    NOT NULL,
		kind          TEXT    NOT NULL,
		original_name TEXT    NOT NULL,
		error_message TEXT    NOT NULL DEFAULT '',
		retry_count   INTEGER NOT NULL DEFAULT 0,
		resolved      INTEGER NOT NULL DEFAULT 0,
		claimed_by    TEXT    NOT NULL DEFAULT '',
		claimed_at    TEXT    NOT NULL DEFAULT '',
		created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("error queue store: create schema: %w", err)
	}
	return nil
}

// Enqueue inserts a failed D-Mail into the error queue.
// Idempotent: re-enqueuing the same name is silently ignored.
func (s *SQLiteErrorQueueStore) Enqueue(name string, data []byte, meta domain.ErrorMetadata) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("error queue store: get conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("error queue store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	_, err = conn.ExecContext(ctx,
		`INSERT OR IGNORE INTO error_queue (name, data, source_outbox, kind, original_name, error_message)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		name, data, meta.SourceOutbox, meta.Kind, meta.OriginalName, meta.Error,
	)
	if err != nil {
		return fmt.Errorf("error queue store: enqueue %s: %w", name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error queue store: commit enqueue %s: %w", name, err)
	}
	committed = true
	return nil
}

// ClaimPendingRetries atomically claims unclaimed pending entries for the
// given claimer. Entries already claimed by another daemon within the last
// 5 minutes are skipped. This prevents duplicate processing across concurrent
// daemon instances.
func (s *SQLiteErrorQueueStore) ClaimPendingRetries(claimerID string, maxRetries int) ([]domain.ErrorEntry, error) {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("error queue store: get conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return nil, fmt.Errorf("error queue store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	// Atomically claim unclaimed rows (or rows with expired claims > 5 min).
	_, err = conn.ExecContext(ctx,
		`UPDATE error_queue
		 SET claimed_by = ?, claimed_at = datetime('now')
		 WHERE resolved = 0 AND retry_count < ?
		   AND (claimed_by = '' OR claimed_at < datetime('now', '-5 minutes'))`,
		claimerID, maxRetries,
	)
	if err != nil {
		return nil, fmt.Errorf("error queue store: claim update: %w", err)
	}

	// Read back claimed rows.
	rows, err := conn.QueryContext(ctx,
		`SELECT name, data, source_outbox, kind, error_message, retry_count
		 FROM error_queue WHERE claimed_by = ? AND resolved = 0`,
		claimerID,
	)
	if err != nil {
		return nil, fmt.Errorf("error queue store: claim select: %w", err)
	}

	var entries []domain.ErrorEntry
	for rows.Next() {
		var e domain.ErrorEntry
		if err := rows.Scan(&e.Name, &e.Data, &e.SourceOutbox, &e.Kind, &e.ErrorMessage, &e.RetryCount); err != nil {
			rows.Close()
			return nil, fmt.Errorf("error queue store: scan row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error queue store: rows iter: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return nil, fmt.Errorf("error queue store: commit claim: %w", err)
	}
	committed = true
	return entries, nil
}

// PendingCount returns the number of unresolved entries below maxRetries.
// This is a read-only operation (READ MODEL) that does not change state.
func (s *SQLiteErrorQueueStore) PendingCount(maxRetries int) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM error_queue WHERE resolved = 0 AND retry_count < ?`,
		maxRetries,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error queue store: pending count: %w", err)
	}
	return count, nil
}

// IncrementRetry increments the retry_count and updates the error message.
// Also resets claimed_by so the entry becomes claimable again.
func (s *SQLiteErrorQueueStore) IncrementRetry(name string, newError string) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("error queue store: get conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("error queue store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	_, err = conn.ExecContext(ctx,
		`UPDATE error_queue
		 SET retry_count = retry_count + 1, error_message = ?,
		     claimed_by = '', claimed_at = ''
		 WHERE name = ?`,
		newError, name,
	)
	if err != nil {
		return fmt.Errorf("error queue store: increment retry %s: %w", name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error queue store: commit increment %s: %w", name, err)
	}
	committed = true
	return nil
}

// MarkResolved marks an entry as resolved (successfully retried).
func (s *SQLiteErrorQueueStore) MarkResolved(name string) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("error queue store: get conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("error queue store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	_, err = conn.ExecContext(ctx, `UPDATE error_queue SET resolved = 1 WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("error queue store: mark resolved %s: %w", name, err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("error queue store: commit resolved %s: %w", name, err)
	}
	committed = true
	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteErrorQueueStore) Close() error {
	return s.db.Close()
}
