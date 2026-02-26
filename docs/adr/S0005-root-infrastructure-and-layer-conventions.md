# S0005. Root Infrastructure Pattern and Layer Conventions

**Date:** 2026-02-26
**Status:** Accepted — supersedes S0001, S0004

## Context

S0001 framed Logger as an "exception" to the types-only root convention.
S0004 placed Tracer (noop variable) in root as a normal convention, not an
exception. Both Logger and Tracer follow the identical pattern: an
infrastructure type/variable in root with noop or injected-I/O defaults,
where real I/O decisions are deferred to the cmd layer. Calling one an
"exception" and the other "normal" is contradictory.

S0004 also established directory naming (`internal/session/`), dependency
direction, and telemetry placement. This ADR consolidates all layer
architecture conventions into a single reference.

## Decision

### Root Package Definition

The root package holds:

1. **Types, constants, pure functions, go:embed** — the original scope
2. **Infrastructure variables and types with noop/zero-value defaults** —
   Logger, Tracer, and similar infrastructure that all layers depend on

Infrastructure in root is NOT an exception. It follows a general pattern:

- The type/variable itself performs no I/O at initialization
- Real I/O targets (io.Writer, OTLP endpoint) are injected or configured
  by the cmd layer at startup
- All layers import root to use these infrastructure types

Examples:
- `Logger`: `NewLogger(io.Writer)` — pure constructor, I/O delegated to Writer
- `Tracer`: `var Tracer = noop.Tracer("tool")` — noop default, no I/O

### I/O Layer Directory Naming

The standard name for the I/O layer is `internal/session/`.

sightjack's additional layers (`internal/domain/`, `internal/eventsource/`)
are tool-specific extensions valid when codebase scale justifies them.

### Dependency Direction

```
internal/cmd  -->  internal/session  -->  root (types, pure fn, go:embed, infra)
```

### Infrastructure Placement Rules

| What | Where | Why |
|------|-------|-----|
| Logger type + NewLogger() | root | Pure constructor, I/O via injected Writer |
| Tracer variable (noop) | root | Noop default, exported for all layers |
| InitTracer() | cmd (or session) | Performs OTLP HTTP connection = I/O |
| NewLogger call site (os.Stderr) | cmd | I/O target decision = cmd responsibility |

### Boundary Test

To determine if something belongs in root: "Does calling this function
or initializing this variable perform I/O (file, network, subprocess)
at call time?" If yes, it belongs in session or cmd. If no (noop, pure
constructor, injected dependency), it may live in root.

## Consequences

### Positive
- Eliminates the false distinction between Logger ("exception") and Tracer ("normal")
- Single ADR for all layer conventions reduces cross-referencing
- Clear boundary test for root-vs-session placement decisions
- Consistent directory naming across all four tools

### Negative
- Supersedes two existing ADRs, requiring README updates across all tools

### Neutral
- No code changes required by this ADR itself; telemetry alignment is
  a separate structural refactoring tracked per-tool
- sightjack's 4-layer architecture remains compatible
