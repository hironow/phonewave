# S0004. Layer Architecture Naming and Telemetry Conventions

**Date:** 2026-02-26
**Status:** Superseded by S0005

## Context

All four tools adopted 2-layer separation (phonewave 0010, sightjack 0011-0012,
paintress 0013, amadeus 0016) but diverged on two points:

1. **I/O layer naming**: phonewave uses `internal/service/`, the other three use
   `internal/session/`. The tools reference each other as prior art but use
   different directory names for the identical architectural role.

2. **Telemetry placement**: Four different approaches emerged:
   - phonewave: `internal/service/telemetry.go`
   - sightjack: `internal/cmd/telemetry.go` (unexported)
   - paintress: `internal/session/telemetry.go` + root noop shim
   - amadeus: root `telemetry.go` (no move)

   Shared ADR 0003 specifies "OTel noop-default + OTLP HTTP" but does not
   prescribe layer placement.

## Decision

### I/O Layer Directory Naming

The standard name for the I/O layer is `internal/session/`.

phonewave's `internal/service/` is renamed to `internal/session/` for
consistency. The 2-layer architecture itself (types-only root, I/O in
internal) is unchanged.

sightjack's additional layers (`internal/domain/`, `internal/eventsource/`)
are tool-specific extensions. The 4-layer architecture is valid when the
codebase scale justifies it (ADR 0011).

### Dependency Direction (canonical)

```
internal/cmd  -->  internal/session  -->  root (types, pure fn, go:embed)
```

### Telemetry Placement Convention

- **Tracer variable**: Root package, initialized to noop. Exported so session
  and cmd can create spans. Example: `var Tracer trace.Tracer = noop.Tracer("tool")`
- **InitTracer()**: `internal/cmd/` (called from main.go or PersistentPreRunE).
  This function performs I/O (OTLP HTTP connection) and should not live in root.
  It may live in session if the tool's architecture makes cmd-level placement
  impractical.

Tools that have not yet aligned with this convention may do so in future
structural refactoring commits. Existing per-tool ADRs documenting different
placement remain valid historical records.

## Consequences

### Positive
- Consistent directory naming across all four tools reduces cognitive overhead
- Developers can navigate any tool's codebase with the same mental model
- Telemetry convention provides clear guidance for new tools or major refactors

### Negative
- phonewave requires a directory rename (`service` -> `session`) with import
  path updates across all source files
- Tools with existing telemetry placement may defer alignment to future work

### Neutral
- sightjack's 4-layer architecture is compatible (session layer exists alongside
  domain and eventsource)
- The Tracer-in-root pattern is already adopted by paintress and amadeus
