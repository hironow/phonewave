# 0000. Cross-Tool Decision Index

**Date:** 2026-02-23
**Status:** Accepted

## Context

The phonewave ecosystem consists of four CLI tools that share common architectural
decisions: phonewave (courier daemon), sightjack (issue scanner), paintress
(autonomous implementer), and amadeus (integrity verifier). These tools were
developed in parallel and converged on shared patterns through cross-tool review
(MY-329, MY-339).

Recording shared decisions in every repository would create duplication and
divergence risk. A single canonical source with cross-references avoids this.

## Decision

Adopt **Option C (hybrid)** for cross-tool ADR management:

1. **phonewave** holds the canonical version of shared ADRs (0001-0005).
2. Each tool maintains its own `docs/adr/` with independent numbering (0001~).
3. Tool-specific ADRs (0006+) live only in the relevant repository.
4. Cross-references use Linear issue numbers (MY-xxx) as stable identifiers.
5. Each tool includes a copy of this index file (`0000-cross-tool-decisions.md`).
6. **S-prefix series**: Shared ADRs added after the 0006+ tool-specific ranges
   were established use `S0001`, `S0002`, ... to avoid numbering collisions.
   These document cross-tool patterns that were independently discovered in
   multiple tools and subsequently recognized as shared conventions.

## Cross-Tool ADR Index (0001-0005)

| # | Decision | Canonical (phonewave) | Applies to | Linear |
|---|----------|-----------------------|------------|--------|
| 0001 | cobra CLI framework adoption | `docs/adr/0001-cobra-cli-framework.md` | all 4 tools | MY-329 |
| 0002 | stdio convention (stdout=data, stderr=logs) | `docs/adr/0002-stdio-convention.md` | all 4 tools | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | `docs/adr/0003-opentelemetry-noop-default.md` | all 4 tools | — |
| 0004 | D-Mail Schema v1 specification | `docs/adr/0004-dmail-schema-v1.md` | all 4 tools | MY-352 |
| 0005 | fsnotify-based file watch daemon | `docs/adr/0005-fsnotify-daemon-design.md` | phonewave, sightjack, paintress | — |

Note: ADR 0005 does not apply to amadeus (CLI-only, no daemon/watcher).

## Extended Shared ADRs (S-series)

Shared ADRs added after tool-specific 0006+ ranges were established use an
S-prefix to avoid numbering collisions. Canonical versions live in phonewave.

| # | Decision | Canonical (phonewave) |
|---|----------|-----------------------|
| S0001 | ~~Logger as root package exception~~ (superseded by S0005) | `docs/adr/S0001-logger-root-package-exception.md` |
| S0002 | JSONL append-only event sourcing pattern | `docs/adr/S0002-event-sourcing-jsonl-pattern.md` |
| S0003 | Three-way approval contract | `docs/adr/S0003-three-way-approval-contract.md` |
| S0004 | ~~Layer architecture conventions~~ (superseded by S0005) | `docs/adr/S0004-layer-architecture-conventions.md` |
| S0005 | Root infrastructure pattern and layer conventions | `docs/adr/S0005-root-infrastructure-and-layer-conventions.md` |

## Tool-Specific ADR Ranges

| Tool | Repository | 0006+ Scope |
|------|-----------|-------------|
| phonewave | `phonewave` | goreleaser, Docker E2E, signal propagation, config-relative state |
| sightjack | `sightjack` | Unix pipe architecture, convergence gate, fake-Claude E2E, Matrix Navigator |
| paintress | `paintress` | Expedition system, per-worker flag isolation, approval contract |
| amadeus | `amadeus` | Pipeline architecture, scoring system, convergence detection, severity routing |

## Consequences

### Positive

- Single source of truth for shared decisions eliminates drift
- Each tool retains autonomy for tool-specific decisions
- Linear issue numbers provide stable cross-references across repositories

### Negative

- phonewave bears the maintenance burden for shared ADR updates
- Other tools must check phonewave for shared decision changes
