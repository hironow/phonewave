# S0009. Daemon SQLite Flush Authority

**Date:** 2026-02-28
**Status:** Accepted

## Context

The transactional outbox pattern (ref: root CLAUDE.md) requires that D-Mail files
are atomically written to both archive/ and outbox/ via SQLite-backed staging.
Multiple CLI instances may run concurrently across different repositories, and the
phonewave daemon watches outbox/ directories for new D-Mail files.

The key tension is between concurrent CLI processes performing Stage + Flush
and the daemon consuming outbox/ contents. SQLite WAL mode with busy_timeout
provides serialized writes, but concurrent Flush operations introduce a subtle
ordering problem: a CLI Flush may interleave with the daemon's outbox scan,
leading to partial reads or duplicate deliveries.

## Decision

Adopt a phased approach to Flush authority:

**Phase 1 (current):** CLI processes perform both Stage and Flush directly.
SQLite WAL mode with `busy_timeout=5000` and `journal_mode=WAL` provides
sufficient concurrency for typical usage (1-3 concurrent CLI instances).
At-least-once delivery is acceptable per root CLAUDE.md ("at least one を許容して
問題がないシンプルな実装に倒す").

**Phase 2 (future, when needed):** The phonewave daemon becomes the sole Flush
authority. CLI processes only Stage (insert into SQLite pending table). The daemon
polls the pending table and performs Flush (write to archive/ + outbox/) as a
single-writer. This eliminates all concurrent Flush races.

**Phase 3 (future, if needed):** IPC-based separation where CLI sends Stage
requests to the daemon via Unix domain socket or named pipe, removing direct
SQLite access from CLI processes entirely.

The transition trigger for Phase 2 is: observing duplicate deliveries or SQLite
busy errors in production telemetry.

## Consequences

### Positive
- Phase 1 is simple: no daemon dependency for basic D-Mail operations
- At-least-once + idempotent receivers means duplicates are harmless
- Clear migration path that does not require breaking changes
- Each phase narrows the write authority, reducing race conditions

### Negative
- Phase 1 accepts potential duplicate deliveries (mitigated by idempotent receivers)
- Phase 2 introduces daemon as critical path for D-Mail delivery
- Phase 2/3 require daemon to be running for outbox Flush to occur

### Neutral
- SQLite WAL mode is already configured in all tools' OutboxStore implementations
- The daemon already watches outbox/ directories via fsnotify
- All D-Mail receivers (inbox consumers) must be idempotent regardless of phase
