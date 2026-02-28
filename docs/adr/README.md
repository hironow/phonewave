# Architecture Decision Records

## Numbering Scheme

| Range | Scope | Description |
|-------|-------|-------------|
| 0000-0009 | Shared (canonical: phonewave) | Cross-tool decisions. All 4 tools follow these. |
| 0006+ (per tool) | Tool-specific | Each tool numbers its own ADRs starting from 0006. |
| S00XX | Shared additions (canonical: phonewave) | Post-initial shared decisions added during alignment. |

- **Shared ADRs** live only in phonewave `docs/adr/`. Other tools reference them but do not copy them.
- **Tool-specific ADRs** live in each tool's own `docs/adr/` with numbering starting at 0006.
- **S-series ADRs** are shared decisions added after the initial 0000-0005 set. They also live only in phonewave.
- Semgrep rules enforcing shared ADRs are copied to each tool's `.semgrep/adr.yaml`.

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

## phonewave-specific ADRs

| # | Decision | Linear |
|---|----------|--------|
| [0006](0006-goreleaser-multiplatform-release.md) | goreleaser multiplatform release strategy | MY-363 |
| [0007](0007-testcontainers-docker-e2e.md) | testcontainers-go Docker E2E testing strategy | MY-363 |
| [0008](0008-signal-context-propagation.md) | Signal context propagation and daemon lifecycle | MY-363 |
| [0009](0009-config-relative-state-directory.md) | Config-relative state directory | MY-363 |
