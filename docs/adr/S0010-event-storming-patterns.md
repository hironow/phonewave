# S0010. Event Storming Pattern Adoption

**Date:** 2026-02-28
**Status:** Accepted

## Context

The 4-tool ecosystem (phonewave, sightjack, paintress, amadeus) uses event sourcing
(S0004) with well-formed EVENTs (past tense, typed envelopes). However, the
Event Storming methodology defines six elements beyond EVENTs:

1. **COMMAND** — an intent to change state (present tense: "will do X")
2. **EVENT** — a recorded state change (past tense: "X happened")
3. **POLICY** — automated reaction: "WHEN [EVENT] THEN [COMMAND]"
4. **READ MODEL** — materialized view built from EVENTs for human consumption
5. **AGGREGATE** — domain model that receives COMMANDs and produces EVENTs
6. **EXTERNAL SYSTEM** — infrastructure accessed by use cases (Claude, git, filesystem)

Current conformance (audited 2026-02-27):
- EVENT: 80% (well-formed in sj/am/pt; pw has no event sourcing)
- COMMAND: 5% (all implicit in cobra handlers, no typed COMMAND objects)
- POLICY: 25% (only pw routing rules; am Projector is partial)
- READ MODEL: 60% (sj/am have projections; pt is missing)
- AGGREGATE: 30% (anemic domain model — pure functions only, no COMMAND→EVENT transition)
- EXTERNAL SYSTEM: 95% (properly separated in session layer)

The gap is primarily in COMMAND types and POLICY definitions. Without typed COMMANDs,
the boundary between "what the user requested" and "what the system did" is blurred,
making it harder to audit, replay, and evolve the system.

## Decision

Adopt Event Storming patterns incrementally across all 4 tools:

### COMMAND Types

Each tool defines typed COMMAND structs in its root package. COMMANDs are:
- Independent of cobra (framework concern separation)
- Validated before execution (Validate method returns []error)
- Converted from cobra flags/args at the cmd layer boundary
- Passed to session-layer use cases, never to domain functions directly

Naming convention: `{Verb}{Noun}Command` (e.g., `RunScanCommand`, `ExecuteCheckCommand`)

Structure:
```go
type RunScanCommand struct {
    RepoPath   string
    Lang       string
    Strictness StrictnessLevel
    DryRun     bool
}

func (c *RunScanCommand) Validate() []error { ... }
```

### POLICY Definitions

POLICYs are expressed as typed rules in the root package:
- Format: `WHEN {EventType} THEN {CommandType}`
- phonewave routing rules are the canonical POLICY example
- Each POLICY has a name, trigger event type, and resulting command type
- POLICYs are registered, not hardcoded in control flow

### AGGREGATE Boundaries

AGGREGATEs wrap domain logic that transitions COMMANDs to EVENTs:
- sightjack: Wave aggregate (ScanCommand → wave.applied, wave.failed)
- paintress: Expedition aggregate (ExpeditionCommand → expedition.completed, expedition.failed)
- amadeus: Check aggregate (CheckCommand → check.completed)
- phonewave: Route aggregate (DeliverCommand → dmail.delivered, dmail.failed)

AGGREGATEs live in the root package as methods on domain types.
They must not perform I/O — that is the session layer's responsibility.

### READ MODEL Projections

READ MODELs are materialized views built by replaying EVENTs:
- Already implemented as Projector/ProjectionStore in sj/am
- paintress needs a READ MODEL for expedition history
- phonewave needs a READ MODEL for delivery status

## Consequences

### Positive
- Clear separation of intent (COMMAND) from outcome (EVENT)
- Auditable: every action has a typed COMMAND that can be logged
- Testable: COMMANDs can be validated in isolation
- Evolvable: new POLICYs can be added without modifying existing code
- Consistent vocabulary across all 4 tools

### Negative
- COMMAND types add boilerplate to each tool's root package
- cobra→COMMAND conversion adds an indirection layer in cmd/
- Incremental adoption means mixed patterns during transition

### Neutral
- Existing EVENTs and EventStore are unchanged
- Session layer use cases gain COMMAND parameters but their I/O behavior is unchanged
- This ADR is a foundation — individual tool adoption is tracked separately
