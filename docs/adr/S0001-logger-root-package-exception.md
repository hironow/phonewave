# S0001. Logger as Root Package Exception

**Date:** 2026-02-25
**Status:** Accepted

## Context

All four tools (phonewave, sightjack, paintress, amadeus) enforce a "root
package is types-only" convention (or aspire to it). However, each tool's
`Logger` type — defined in `logger.go` at the package root — performs I/O
through an `io.Writer` parameter. By strict interpretation, this violates the
types-only rule.

Moving `Logger` to an internal sub-package was evaluated independently in
multiple tools:

- phonewave ADR 0010: "logger.go stays in root as an intentional exception —
  23+ dependents make migration cost exceed benefit"
- sightjack ADR 0012: Listed `logger.go` in "Items explicitly NOT moved" with
  the same rationale
- amadeus ADR 0014: Flat architecture retains Logger by default

In every case the conclusion was identical: the migration cost (import-prefix
churn across 20+ files) far exceeds the architectural benefit.

## Decision

Recognize `logger.go` in the root package as a **shared exception** to the
types-only root convention across all four tools.

1. Each tool keeps `Logger` type and its methods in `logger.go` at the package
   root.
2. `Logger` writes to an injected `io.Writer` — it does not open files or
   perform network I/O on its own (deferred I/O pattern).
3. If a future tool's Logger grows to perform direct I/O (e.g., file rotation),
   it should be moved to the I/O layer and this ADR should be superseded.

## Consequences

### Positive
- Eliminates repeated justification of the same exception in each tool's ADR
- Establishes a clear precedent: infrastructure types with 20+ dependents may
  remain in root when the migration cost exceeds the benefit

### Negative
- The exception weakens the "types-only root" rule — future developers must
  understand that Logger is a special case, not a precedent for other I/O types

### Neutral
- This ADR documents an existing convention; no code changes are required
