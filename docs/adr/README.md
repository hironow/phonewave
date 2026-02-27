# Architecture Decision Records

## Shared ADRs (canonical: phonewave)

phonewave holds the canonical version of these ADRs for the 4-tool ecosystem
(phonewave, sightjack, paintress, amadeus). Other tools reference these but
do not copy them.

| # | Decision | Linear |
|---|----------|--------|
| [0000](0000-cross-tool-decisions.md) | Cross-Tool Decision Index | MY-363 |
| [0001](0001-cobra-cli-framework.md) | cobra CLI framework adoption | MY-329 |
| [0002](0002-stdio-convention.md) | stdio convention (stdout=data, stderr=logs) | MY-339 |
| [0003](0003-opentelemetry-noop-default.md) | OpenTelemetry noop-default + OTLP HTTP | MY-363 |
| [0004](0004-dmail-schema-v1.md) | D-Mail Schema v1 specification | MY-352, MY-353 |
| [0005](0005-fsnotify-daemon-design.md) | fsnotify-based file watch daemon | MY-363 |

## Extended Shared ADRs (S-series, canonical: phonewave)

Cross-tool patterns independently discovered in multiple tools,
subsequently recognized as shared conventions.

| # | Decision |
|---|----------|
| [S0001](S0001-logger-root-package-exception.md) | ~~Logger as root package exception~~ (superseded by S0005) |
| [S0002](S0002-event-sourcing-jsonl-pattern.md) | JSONL append-only event sourcing pattern |
| [S0003](S0003-three-way-approval-contract.md) | Three-way approval contract |
| [S0004](S0004-layer-architecture-conventions.md) | ~~Layer architecture conventions~~ (superseded by S0005) |
| [S0005](S0005-root-infrastructure-and-layer-conventions.md) | Root infrastructure pattern and layer conventions |

## phonewave-specific ADRs

| # | Decision | Linear |
|---|----------|--------|
| [0006](0006-goreleaser-multiplatform-release.md) | goreleaser multiplatform release strategy | MY-363 |
| [0007](0007-testcontainers-docker-e2e.md) | testcontainers-go Docker E2E testing strategy | MY-363 |
| [0008](0008-signal-context-propagation.md) | Signal context propagation and daemon lifecycle | MY-363 |
| [0009](0009-config-relative-state-directory.md) | Config-relative state directory | MY-363 |
| [0010](0010-root-package-layer-separation.md) | Root package layer separation | — |
