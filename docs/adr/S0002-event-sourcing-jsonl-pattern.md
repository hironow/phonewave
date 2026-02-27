# S0002. JSONL Append-Only Event Sourcing Pattern

**Date:** 2026-02-25
**Status:** Accepted

## Context

sightjack (ADR 0008, 0009) and amadeus (ADR 0011, 0013) independently adopted
event sourcing for state management. Both tools converged on a nearly identical
architecture despite being designed without cross-tool coordination:

- JSONL append-only files as the single source of truth
- Event envelope with type/timestamp/data fields
- Event validation at append time (before any file writes)
- Lifecycle management via archive-prune CLI command
- No snapshots, CQRS, or schema evolution (YAGNI for CLI tools with < 100
  events per session)

This convergence indicates a stable, reusable pattern worth documenting as a
shared convention.

## Decision

Recognize JSONL append-only event sourcing as the shared pattern for file-based
state management in the phonewave ecosystem.

### Core Pattern

1. **JSONL append-only store**: Events are written to `.jsonl` files using
   `O_APPEND|O_CREATE|O_WRONLY`. Each line is one JSON-encoded event. No
   read-modify-write cycles.

2. **Event envelope**: Every event has at minimum: identifier, type, timestamp,
   and typed data payload. The identifier format is tool-specific.

3. **Append-time validation**: `ValidateEvent()` checks structural validity
   (non-empty required fields) before persisting. Invalid events reject the
   entire batch.

4. **Per-file fsync**: After writing events, `f.Sync()` flushes the OS buffer
   to disk for crash durability.

5. **Projection**: Current state is derived by replaying events. Eager
   projection (write-time) is preferred for CLI tools where read latency
   matters.

6. **Lifecycle pruning**: Old event files are cleaned up alongside D-Mail
   archives via the `archive-prune` CLI command with a configurable retention
   threshold.

### Tool-Specific Trade-offs

| Aspect | sightjack | amadeus |
|--------|-----------|---------|
| **File scope** | Per-session (`events/{sessionID}.jsonl`) | Daily rotation (`events/YYYY-MM-DD.jsonl`) |
| **Event ID** | Sequential integer (monotonicity check) | UUID v4 (`google/uuid`) |
| **Ordering guarantee** | In-memory `lastWrittenSeq` monotonicity | Timestamp-based ordering |
| **Projection** | Lazy (replay on read) | Eager (write-time) + lazy rebuild |
| **ADRs** | 0008 (pattern), 0009 (validation + concurrency) | 0011 (pattern), 0013 (validation + lifecycle) |

### Explicitly Not Required

- **Snapshots**: Event counts (< 100 per session) make full replay < 1ms
- **CQRS**: Single process, single projection
- **Schema evolution / upcasting**: Not needed pre-release; old files can be
  deleted
- **Correlation / Causation IDs**: Single-process CLI, no distributed tracing

## Consequences

### Positive

- Shared vocabulary for state management across tools that adopt event sourcing
- New tools can reference this ADR instead of re-deriving the pattern
- Tool-specific trade-offs are documented in one place for comparison

### Negative

- Tools must still write their own tool-specific ADRs for implementation
  decisions (file scope, ID format, projection strategy)

### Neutral

- phonewave and paintress do not currently use event sourcing; this ADR does
  not mandate adoption
