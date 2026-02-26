# 0010. Root Package Layer Separation

**Date:** 2026-02-25
**Status:** Accepted — directory renamed to internal/session/ per S0004

## Context

phonewave's root package (`phonewave`) contained both type definitions and
I/O operations (filesystem, network, subprocess). This created a monolithic
package where pure domain types and side-effecting functions lived together,
making it harder to reason about dependency direction and testability.

sightjack ADRs 0010 (File Consolidation), 0011 (Layer-First Refactoring),
and 0012 (Root I/O Cleanup) established a pattern of separating types from
I/O. phonewave needed to conform to these conventions.

Go prohibits circular imports: since `internal/service` imports root for
types, root cannot import `internal/service`. This constraint requires all
I/O to move atomically.

## Decision

Separate the root package into two layers:

- **Root `phonewave`**: types, constants, and pure functions only. No I/O.
- **`internal/service`**: all filesystem, network, and subprocess I/O.

This is a 2-layer architecture (not sightjack's 3-layer) because a separate
domain layer is YAGNI for phonewave's size.

`logger.go` stays in root as an intentional exception — 23+ dependents make
migration cost exceed benefit.

Dependency direction: `internal/cmd` → `internal/service` → `phonewave` (root).

## Consequences

### Positive
- Root package is safe to import without pulling in I/O dependencies
- Clear separation of concerns: types vs operations
- Aligns with sightjack ADR conventions (0010-0012)
- Tests split naturally: pure function tests in root, I/O tests in service

### Negative
- Larger initial commit (atomic move required by Go circular import constraint)
- `internal/cmd` now imports two packages instead of one
- `logger.go` exception breaks the pure types-only rule for root

### Neutral
- Report types (`DoctorReport`, `StatusReport`, `DaemonOptions`) live in
  `internal/service` since they are generated and consumed there
