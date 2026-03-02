# S0020. Accepted Cross-Tool Divergence

**Date:** 2026-03-02
**Status:** Accepted

## Context

Cross-tool gap inventory (2026-03-01, 2026-03-02) identified two structural
differences across the four CLI tools (phonewave, sightjack, paintress,
amadeus). Both were reviewed via codex and determined to be intentional design
choices rooted in each tool's domain semantics, not accidental drift.

Unifying these differences would either distort tool semantics (GAP-01-01)
or introduce data loss risk (GAP-03-01).

## Decision

Accept the following divergences as intentional design and document them
in each tool's CLAUDE.md for discoverability.

### GAP-01-01: Default Subcommand Name

Each tool auto-prepends a different default subcommand matching its primary
use case:

| Tool | Default | Function | Rationale |
|------|---------|----------|-----------|
| phonewave | `run` | `NeedsDefaultRun` | Daemon execution |
| sightjack | `scan` | `NeedsDefaultScan` | Issue inspection |
| paintress | `run` | `NeedsDefaultRun` | Autonomous expedition |
| amadeus | `check` | `NeedsDefaultCheck` | Integrity verification |

The function signature contract is unified: `NeedsDefault<Verb>(rootCmd, args) bool`.
Only the verb differs, reflecting each tool's domain.

Note: paintress returns `false` for empty args because its `run` subcommand
requires `ExactArgs(1)` (repository path). This is intentional — auto-prepending
`run` with no args would produce an "insufficient arguments" error instead of
showing help.

### GAP-03-01: Eventsource Storage Model and Pruning

| Tool | Storage Model | Prune Method | Rationale |
|------|--------------|--------------|-----------|
| phonewave | flat `.jsonl` | `os.Remove` | Single file per event stream |
| sightjack | per-session directories | `os.RemoveAll` | Session-scoped event isolation |
| paintress | flat `.jsonl` | `os.Remove` | Single file per event stream |
| amadeus | flat `.jsonl` | `os.Remove` | Single file per event stream |

Sightjack uses per-session directories (`events/{sessionID}/`) because scan
sessions are independent units that benefit from filesystem-level isolation.
The other three tools use flat `.jsonl` files where `os.Remove` is sufficient
and safer (cannot accidentally delete directory trees).

All four tools share the `eventsource.EventsDir(stateDir)` helper for path
construction. Sightjack's `ListExpiredEventFiles` includes a safety filter
(dirs + `.jsonl` only) to prevent accidental deletion of unexpected entries.

## Consequences

### Positive

- Each tool's CLI matches its domain vocabulary (run/scan/check)
- Storage model matches each tool's concurrency and isolation requirements
- Safety filter in sightjack prevents accidental data loss from unexpected files
- Unified function signature and EventsDir helper reduce cognitive load

### Negative

- New contributors must learn that verb names differ intentionally
- Storage model difference means eventsource code is not 100% identical
