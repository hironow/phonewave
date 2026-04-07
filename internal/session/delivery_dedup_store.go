package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/phonewave/internal/usecase/port"

	_ "modernc.org/sqlite"
)

// Compile-time check that SQLiteDeliveryDedupStore implements port.DeliveryDedupStore.
var _ port.DeliveryDedupStore = (*SQLiteDeliveryDedupStore)(nil)

// SQLiteDeliveryDedupStore provides exact dedup for D-Mail delivery using SQLite.
// Unlike the Bloom filter (probabilistic, false positives possible), this store
// provides exact-match dedup to prevent message loss.
type SQLiteDeliveryDedupStore struct {
	db *sql.DB
}

// NewSQLiteDeliveryDedupStore opens (or creates) a delivery dedup database.
func NewSQLiteDeliveryDedupStore(dbPath string) (*SQLiteDeliveryDedupStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("delivery dedup store: create dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath) // nosemgrep: d4-sql-open-without-defer-close -- stored in struct, closed via Close() [permanent]
	if err != nil {
		return nil, fmt.Errorf("delivery dedup store: open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("delivery dedup store: set WAL: %w", err)
	}

	if err := createDeliveryDedupSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("delivery dedup store: create schema: %w", err)
	}

	return &SQLiteDeliveryDedupStore{db: db}, nil
}

func createDeliveryDedupSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS delivery_log (
		idempotency_key TEXT NOT NULL,
		target          TEXT NOT NULL,
		delivered_at    TEXT NOT NULL,
		PRIMARY KEY (idempotency_key, target)
	)`)
	return err
}

// HasDelivered returns true if a D-Mail with the given idempotency key
// has already been delivered to the specified target.
func (s *SQLiteDeliveryDedupStore) HasDelivered(ctx context.Context, idempotencyKey, target string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM delivery_log WHERE idempotency_key = ? AND target = ?`,
		idempotencyKey, target).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("delivery dedup: has delivered: %w", err)
	}
	return count > 0, nil
}

// RecordDelivery records a successful delivery. Duplicate keys are silently ignored.
func (s *SQLiteDeliveryDedupStore) RecordDelivery(ctx context.Context, idempotencyKey string, target string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO delivery_log (idempotency_key, target, delivered_at) VALUES (?, ?, ?)`,
		idempotencyKey, target, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("delivery dedup: record: %w", err)
	}
	return nil
}

// Close releases database resources.
func (s *SQLiteDeliveryDedupStore) Close() error {
	return s.db.Close()
}
