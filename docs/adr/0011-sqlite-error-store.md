# 0011. SQLite Error Store for Failed D-Mail Deliveries

**Date:** 2026-02-26
**Status:** Accepted

## Context

phonewave's error queue stored failed D-Mail deliveries as pairs of files
in `.phonewave/errors/`: a data file (raw D-Mail content) and a `.err`
YAML sidecar (metadata: attempts, error message, timestamps). This
filesystem-based approach had several problems:

1. **Non-atomic writes** — writing the data file and sidecar required two
   separate `os.WriteFile` calls. If the process crashed between them, an
   orphaned data file with no metadata remained.
2. **Not concurrent-safe** — multiple daemon instances (or a daemon and a
   CLI command) could race on the same error directory without coordination.
3. **Inconsistent with other tools** — sightjack, paintress, and amadeus
   all use SQLite with WAL mode for their durable stores (OutboxStore),
   following the transactional outbox pattern defined in the parent
   CLAUDE.md.

phonewave is a consumer (courier): it reads from outbox directories and
delivers to inbox directories. Unlike producers, it does not need a
transactional outbox for staging D-Mails. However, it does need a durable,
atomic store for failed deliveries that will be retried.

## Decision

Replace the filesystem-based error queue with a SQLite-backed
`ErrorStore`. The implementation (`SQLiteErrorStore`) stores all failure
metadata and payload in a single `delivery_errors` table, using the same
SQLite patterns established by the other tools:

- **Driver:** `modernc.org/sqlite` (pure Go, CGo-free)
- **Connection:** `SetMaxOpenConns(1)` — prevents "database is locked"
  within a single process
- **WAL mode:** `PRAGMA journal_mode=WAL` — allows concurrent reads from
  other processes
- **Busy timeout:** `PRAGMA busy_timeout=5000` — tolerates brief lock
  contention from concurrent CLI invocations
- **Write transactions:** `BEGIN IMMEDIATE` for update operations
- **Idempotency:** `INSERT OR IGNORE` for `RecordFailure`

The database file lives at `.phonewave/.run/errors.db`, following the
`.run/` convention for runtime state.

The `ErrorStore` interface is defined in the root package (`interfaces.go`)
as a port, with `SQLiteErrorStore` in `internal/session/` as the adapter.

## Consequences

### Positive

- Recording a failure is a single atomic transaction — no orphaned files
- WAL mode + busy_timeout handles concurrent daemon/CLI access correctly
- Aligns phonewave with the other three tools' infrastructure patterns
- Payload stored in BLOB column — no filesystem naming or sanitization
  concerns

### Negative

- Adds `modernc.org/sqlite` dependency (~5 MB binary size increase), though
  this is already accepted in the other three tools
- SQLite database requires `.run/` directory to exist (handled by
  `EnsureStateDir`)

### Neutral

- The `gopkg.in/yaml.v3` dependency is no longer needed for error metadata
  serialization (sidecar files removed), though it remains for config
  loading
