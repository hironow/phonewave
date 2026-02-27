package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	phonewave "github.com/hironow/phonewave"

	_ "modernc.org/sqlite"
)

// DeliverFunc is the signature for a delivery callback injected into
// SQLiteOutboxStore. It mirrors DeliverData but accepts a reconstructed
// D-Mail path and pre-read data, returning the delivery result.
type DeliverFunc func(ctx context.Context, dmailPath string, data []byte) (*phonewave.DeliveryResult, error)

// Compile-time check that SQLiteOutboxStore implements phonewave.OutboxStore.
var _ phonewave.OutboxStore = (*SQLiteOutboxStore)(nil)

// SQLiteOutboxStore implements OutboxStore using a SQLite database as the
// transactional write-ahead log. Staged D-Mails are flushed by invoking
// the injected delivery function (unlike other tools that write to
// archive/ + outbox/ files).
type SQLiteOutboxStore struct {
	db        *sql.DB
	deliverFn DeliverFunc
}

// NewSQLiteOutboxStore opens (or creates) a SQLite database at dbPath and
// initialises the schema. deliverFn is invoked for each item during Flush.
func NewSQLiteOutboxStore(dbPath string, deliverFn DeliverFunc) (*SQLiteOutboxStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("outbox store: create dir %s: %w", filepath.Dir(dbPath), err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("outbox store: open db: %w", err)
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
			return nil, fmt.Errorf("outbox store: %s: %w", pragma, err)
		}
	}
	if err := createOutboxSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteOutboxStore{
		db:        db,
		deliverFn: deliverFn,
	}, nil
}

func createOutboxSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS staged (
		source_dir TEXT    NOT NULL,
		name       TEXT    NOT NULL,
		kind       TEXT    NOT NULL,
		data       BLOB   NOT NULL,
		flushed    INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (source_dir, name)
	)`)
	if err != nil {
		return fmt.Errorf("outbox store: create schema: %w", err)
	}
	return nil
}

// Stage inserts a D-Mail into the staging table. Idempotent: re-staging the
// same (source_dir, name) pair is silently ignored (INSERT OR IGNORE).
func (s *SQLiteOutboxStore) Stage(name, kind, sourceDir string, data []byte) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO staged (source_dir, name, kind, data) VALUES (?, ?, ?, ?)`,
		sourceDir, name, kind, data,
	)
	if err != nil {
		return fmt.Errorf("outbox store: stage %s: %w", name, err)
	}
	return nil
}

// Flush delivers all unflushed D-Mails via the injected delivery function,
// then marks them as flushed in the database. Uses BEGIN IMMEDIATE to
// prevent concurrent access deadlocks. A partial failure leaves items
// eligible for retry on the next Flush call.
func (s *SQLiteOutboxStore) Flush() (int, error) {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("outbox store: get conn: %w", err)
	}
	defer conn.Close()

	// BEGIN IMMEDIATE acquires a RESERVED lock immediately, preventing
	// the SHARED→EXCLUSIVE deadlock that occurs with DEFERRED transactions
	// when two connections SELECT then UPDATE concurrently.
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return 0, fmt.Errorf("outbox store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	rows, err := conn.QueryContext(ctx,
		`SELECT source_dir, name, kind, data FROM staged WHERE flushed = 0`)
	if err != nil {
		return 0, fmt.Errorf("outbox store: query staged: %w", err)
	}

	type item struct {
		sourceDir string
		name      string
		kind      string
		data      []byte
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.sourceDir, &it.name, &it.kind, &it.data); err != nil {
			rows.Close()
			return 0, fmt.Errorf("outbox store: scan row: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("outbox store: rows iter: %w", err)
	}

	if len(items) == 0 {
		conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		committed = true
		return 0, nil
	}

	flushed := 0
	for _, it := range items {
		dmailPath := filepath.Join(it.sourceDir, it.name)
		if _, deliverErr := s.deliverFn(ctx, dmailPath, it.data); deliverErr != nil {
			continue
		}
		if _, err := conn.ExecContext(ctx,
			`UPDATE staged SET flushed = 1 WHERE source_dir = ? AND name = ?`,
			it.sourceDir, it.name,
		); err != nil {
			return flushed, fmt.Errorf("outbox store: mark flushed %s: %w", it.name, err)
		}
		flushed++
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, fmt.Errorf("outbox store: commit: %w", err)
	}
	committed = true
	return flushed, nil
}

// FlushPending returns the number of staged items that have not yet been flushed.
func (s *SQLiteOutboxStore) FlushPending() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM staged WHERE flushed = 0`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("outbox store: flush pending: %w", err)
	}
	return count, nil
}

// DB returns the underlying database handle. Exposed for testing PRAGMAs.
func (s *SQLiteOutboxStore) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database connection.
func (s *SQLiteOutboxStore) Close() error {
	return s.db.Close()
}
