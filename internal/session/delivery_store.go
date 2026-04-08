package session

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	_ "modernc.org/sqlite"
)

const maxDeliveryRetryCount = 3

// SQLiteDeliveryStore implements port.DeliveryStore using SQLite
// with a 2-phase Flush to minimise lock hold time during filesystem I/O.
type SQLiteDeliveryStore struct {
	db *sql.DB
}

// NewSQLiteDeliveryStore opens (or creates) a SQLite delivery store
// at {stateDir}/.run/delivery.db and initialises the schema.
func NewSQLiteDeliveryStore(stateDir string) (*SQLiteDeliveryStore, error) {
	runDir := filepath.Join(stateDir, ".run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("delivery store: create dir: %w", err)
	}
	dbPath := filepath.Join(runDir, "delivery.db")
	db, err := sql.Open("sqlite", dbPath) // nosemgrep: d4-sql-open-without-defer-close -- stored in struct, closed via Close() [permanent]
	if err != nil {
		return nil, fmt.Errorf("delivery store: open db: %w", err)
	}

	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA auto_vacuum=INCREMENTAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("delivery store: %s: %w", pragma, err)
		}
	}

	if err := createDeliverySchema(db); err != nil {
		db.Close()
		return nil, err
	}

	// Set file permissions after creation
	if err := os.Chmod(dbPath, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("delivery store: chmod: %w", err)
	}

	return &SQLiteDeliveryStore{db: db}, nil
}

func createDeliverySchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS staged_delivery (
		dmail_path  TEXT    NOT NULL,
		target      TEXT    NOT NULL,
		data        BLOB    NOT NULL,
		flushed     INTEGER NOT NULL DEFAULT 0,
		retry_count INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (dmail_path, target)
	)`)
	if err != nil {
		return fmt.Errorf("delivery store: create schema: %w", err)
	}
	return nil
}

// StageDelivery records delivery intents for all targets in a single transaction.
// Idempotent: re-staging the same (dmailPath, target) pair updates the data and
// resets flushed/retry state, enabling re-delivery of recurring D-Mails.
func (s *SQLiteDeliveryStore) StageDelivery(ctx context.Context, dmailPath string, data []byte, targets []string) (stageErr error) {
	ctx, span := platform.Tracer.Start(ctx, "outbox.stage.delivery") // nosemgrep: adr0003-otel-span-without-defer-end [permanent]
	defer func() {
		if stageErr != nil {
			span.RecordError(stageErr)
			span.SetAttributes(attribute.String("error.stage", "outbox.stage.delivery"))
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.operation", "stage"))

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("delivery store: get conn: %w", err)
	}
	defer conn.Close()

	lockStart := time.Now()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("delivery store: begin immediate: %w", err)
	}
	if platform.IsDetailDebug() {
		span.SetAttributes(attribute.Int64("db.lock_wait_ms", time.Since(lockStart).Milliseconds()))
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	for _, target := range targets {
		_, err := conn.ExecContext(ctx,
			`INSERT INTO staged_delivery (dmail_path, target, data) VALUES (?, ?, ?)
			ON CONFLICT(dmail_path, target) DO UPDATE SET data = excluded.data, flushed = 0, retry_count = 0
			WHERE flushed = 0 OR data != excluded.data`,
			dmailPath, target, data,
		)
		if err != nil {
			return fmt.Errorf("delivery store: stage %s → %s: %w", dmailPath, target, err)
		}
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("delivery store: commit stage: %w", err)
	}
	committed = true
	return nil
}

// unflushedItem is an internal struct for the 2-phase Flush approach.
type unflushedItem struct {
	dmailPath string
	target    string
	data      []byte
}

// FlushDeliveries writes unflushed staged items to their target paths.
// Uses a 2-phase approach to minimise SQLite lock hold time:
//  1. Short read transaction to collect unflushed items
//  2. Filesystem I/O (atomicWrite) outside any transaction
//  3. Short write transaction per item to update status
func (s *SQLiteDeliveryStore) FlushDeliveries(ctx context.Context) (results []domain.DeliveryFlushed, flushErr error) {
	ctx, span := platform.Tracer.Start(ctx, "outbox.flush.deliveries") // nosemgrep: adr0003-otel-span-without-defer-end [permanent]
	retryCount := 0
	defer func() {
		if flushErr != nil {
			span.RecordError(flushErr)
			span.SetAttributes(attribute.String("error.stage", "outbox.flush.deliveries"))
		}
		if platform.IsDetailDebug() {
			span.SetAttributes(
				attribute.Int("flush.success.count", len(results)),
				attribute.Int("flush.retry.count", retryCount),
			)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.operation", "flush"))

	// Phase 1: Read unflushed items (short transaction)
	items, err := s.collectUnflushed()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	// Phase 2: Write files (no transaction held)
	var flushed []domain.DeliveryFlushed
	for _, item := range items {
		writeErr := atomicWrite(item.target, item.data)

		// Phase 3: Update status (short transaction per item)
		if writeErr != nil {
			retryCount++
			if err := s.incrementRetryCount(item.dmailPath, item.target); err != nil {
				return flushed, fmt.Errorf("delivery store: increment retry: %w", err)
			}
			continue
		}

		if err := s.markFlushed(item.dmailPath, item.target); err != nil {
			return flushed, fmt.Errorf("delivery store: mark flushed: %w", err)
		}
		flushed = append(flushed, domain.DeliveryFlushed{
			DMailPath: item.dmailPath,
			Target:    item.target,
		})
	}

	return flushed, nil
}

// collectUnflushed reads all unflushed items below max retry count.
func (s *SQLiteDeliveryStore) collectUnflushed() ([]unflushedItem, error) {
	rows, err := s.db.Query(
		`SELECT dmail_path, target, data FROM staged_delivery
		 WHERE flushed = 0 AND retry_count < ?`,
		maxDeliveryRetryCount,
	)
	if err != nil {
		return nil, fmt.Errorf("delivery store: select unflushed: %w", err)
	}
	defer rows.Close()

	var items []unflushedItem
	for rows.Next() {
		var item unflushedItem
		if err := rows.Scan(&item.dmailPath, &item.target, &item.data); err != nil {
			return nil, fmt.Errorf("delivery store: scan unflushed: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// markFlushed sets flushed=1 for a single (dmailPath, target) pair.
func (s *SQLiteDeliveryStore) markFlushed(dmailPath, target string) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	_, err = conn.ExecContext(ctx,
		`UPDATE staged_delivery SET flushed = 1 WHERE dmail_path = ? AND target = ?`,
		dmailPath, target,
	)
	if err != nil {
		return err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// incrementRetryCount increments retry_count for a failed flush attempt.
func (s *SQLiteDeliveryStore) incrementRetryCount(dmailPath, target string) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	_, err = conn.ExecContext(ctx,
		`UPDATE staged_delivery SET retry_count = retry_count + 1 WHERE dmail_path = ? AND target = ?`,
		dmailPath, target,
	)
	if err != nil {
		return err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// RecoverUnflushed returns all unflushed delivery intents.
// Used at startup to detect crash-interrupted deliveries.
func (s *SQLiteDeliveryStore) RecoverUnflushed() ([]domain.StagedDelivery, error) {
	rows, err := s.db.Query(
		`SELECT dmail_path, target, data FROM staged_delivery WHERE flushed = 0`,
	)
	if err != nil {
		return nil, fmt.Errorf("delivery store: recover unflushed: %w", err)
	}
	defer rows.Close()

	var items []domain.StagedDelivery
	for rows.Next() {
		var item domain.StagedDelivery
		if err := rows.Scan(&item.DMailPath, &item.Target, &item.Data); err != nil {
			return nil, fmt.Errorf("delivery store: scan unflushed: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// AllFlushedFor returns true when all targets for the given dmailPath are flushed.
func (s *SQLiteDeliveryStore) AllFlushedFor(dmailPath string) (bool, error) {
	var unflushedCount int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM staged_delivery WHERE dmail_path = ? AND flushed = 0`,
		dmailPath,
	).Scan(&unflushedCount)
	if err != nil {
		return false, fmt.Errorf("delivery store: all flushed check: %w", err)
	}
	return unflushedCount == 0, nil
}

// PruneFlushed deletes all flushed rows and runs incremental vacuum.
func (s *SQLiteDeliveryStore) PruneFlushed(ctx context.Context) (count int, pruneErr error) {
	_, span := platform.Tracer.Start(ctx, "outbox.prune") // nosemgrep: adr0003-otel-span-without-defer-end [permanent]
	defer func() {
		if pruneErr != nil {
			span.RecordError(pruneErr)
			span.SetAttributes(attribute.String("error.stage", "outbox.prune"))
		} else if platform.IsDetailDebug() {
			span.SetAttributes(attribute.Int("prune.count", count))
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.operation", "prune"))

	affected, err := s.deleteFlushedRows()
	if err != nil {
		return 0, err
	}

	// Incremental vacuum outside transaction (conn released by deleteFlushedRows)
	if _, err := s.db.Exec("PRAGMA incremental_vacuum"); err != nil {
		return affected, fmt.Errorf("delivery store: vacuum: %w", err)
	}

	return affected, nil
}

// deleteFlushedRows deletes all flushed rows in a single transaction.
func (s *SQLiteDeliveryStore) deleteFlushedRows() (int, error) {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("delivery store: get conn: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return 0, fmt.Errorf("delivery store: begin immediate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck
		}
	}()

	result, err := conn.ExecContext(ctx, `DELETE FROM staged_delivery WHERE flushed = 1`)
	if err != nil {
		return 0, fmt.Errorf("delivery store: delete flushed: %w", err)
	}

	affected, _ := result.RowsAffected()

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, fmt.Errorf("delivery store: commit prune: %w", err)
	}
	committed = true
	return int(affected), nil
}

// DeadLetterCount returns the number of delivery items that have exceeded maxDeliveryRetryCount.
func (s *SQLiteDeliveryStore) DeadLetterCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM staged_delivery WHERE flushed = 0 AND retry_count >= ?`, maxDeliveryRetryCount).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("delivery store: dead letter count: %w", err)
	}
	return count, nil
}

// PurgeDeadLetters deletes delivery items that have exceeded maxDeliveryRetryCount.
// Returns the number of purged items.
func (s *SQLiteDeliveryStore) PurgeDeadLetters(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM staged_delivery WHERE flushed = 0 AND retry_count >= ?`, maxDeliveryRetryCount)
	if err != nil {
		return 0, fmt.Errorf("delivery store: purge dead letters: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delivery store: rows affected: %w", err)
	}
	return int(deleted), nil
}

// Close closes the underlying database connection.
func (s *SQLiteDeliveryStore) Close() error {
	return s.db.Close()
}
