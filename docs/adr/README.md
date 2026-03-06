# Architecture Decision Records

## Numbering Scheme

| Range | Scope | Description |
|-------|-------|-------------|
| 0000-0005 | Shared (canonical: phonewave) | Cross-tool decisions. All 4 tools follow these. |
| 0006+ (per tool) | Tool-specific | Each tool numbers its own ADRs starting from 0006. |
| S00XX | Shared additions (canonical: phonewave) | Post-initial shared decisions added during alignment. |

- **Shared ADRs (0000-0005)** live only in phonewave `docs/adr/`. Other tools reference them but do not copy them.
- **Tool-specific ADRs (0006+)** live in each tool's own `docs/adr/` with numbering starting at 0006.
- **S-series ADRs** are shared decisions added after the initial 0000-0005 set. They also live only in phonewave.
- Semgrep rules enforcing shared ADRs are copied to each tool's `.semgrep/shared-adr.yaml`.

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

## S-series Shared ADRs (canonical: phonewave)

| # | Decision | Linear |
|---|----------|--------|
| [S0011](S0011-sqlite-wal-cooperative-model.md) | SQLite WAL cooperative model for concurrent CLI | — |
| [S0012](S0012-reference-data-management.md) | Reference data management pattern | — |
| [S0013](S0013-command-naming-convention.md) | COMMAND naming convention (imperative present tense) | — |
| [S0014](S0014-policy-pattern-reference.md) | POLICY pattern reference implementation | — |
| [S0015](S0015-state-directory-naming.md) | State directory naming convention | — |
| [S0016](S0016-root-package-organization.md) | Root package file organization | — |
| [S0017](S0017-aggregate-root-and-usecase-layer.md) | Aggregate root and use case layer | — |
| [S0018](S0018-event-storming-alignment.md) | Event Storming alignment and per-tool applicability | — |
| [S0019](S0019-data-persistence-boundaries.md) | Data persistence boundaries (Linear/GitHub/local) | — |
| [S0020](S0020-accepted-cross-tool-divergence.md) | Accepted cross-tool divergence (default subcommand, storage model) | — |
| [S0021](S0021-dmail-receive-side-postel-law.md) | D-Mail receive-side validation (Postel's Law) | — |
| [S0022](S0022-otel-metrics-design.md) | OTel Metrics Design | — |
| [S0023](S0023-cross-tool-contract-testing.md) | Cross-Tool Contract Testing | — |
| [S0024](S0024-cli-argument-design-decisions.md) | ~~CLI Argument Design Decisions~~ | Superseded by S0028 |
| [S0025](S0025-event-delivery-guarantee-levels.md) | Event Delivery Guarantee Levels | — |
| [S0026](S0026-domain-model-maturity-assessment.md) | Domain Model Maturity Assessment | — |
| [S0027](S0027-rdra-gap-resolution.md) | RDRA Gap Resolution — D-Mail Protocol Extension | — |
| [S0028](S0028-cli-argument-design-actual.md) | CLI Argument Design (Actual Implementation) — supersedes S0024 | — |

## phonewave-specific ADRs

| # | Decision | Linear |
|---|----------|--------|
| [0006](0006-goreleaser-multiplatform-release.md) | goreleaser multiplatform release strategy | MY-363 |
| [0007](0007-testcontainers-docker-e2e.md) | testcontainers-go Docker E2E testing strategy | MY-363 |
| [0008](0008-signal-context-propagation.md) | Signal context propagation and daemon lifecycle | MY-363 |
| [0009](0009-config-relative-state-directory.md) | Config-relative state directory | MY-363 |
