# Architecture Decision Records

## Shared (cross-tool canonical)

phonewave holds the canonical version of these ADRs for the 4-tool ecosystem
(phonewave, sightjack, paintress, amadeus). Other tools reference these but
do not copy them.

| # | Decision |
|---|----------|
| 0000 | Cross-Tool Decision Index |
| 0001 | cobra CLI framework adoption |
| 0002 | stdio convention (stdout=data, stderr=logs) |
| 0003 | OpenTelemetry noop-default + OTLP HTTP |
| 0004 | D-Mail Schema v1 specification |
| 0005 | fsnotify-based file watch daemon |

## phonewave-specific

| # | Decision |
|---|----------|
| 0006 | goreleaser multiplatform release strategy |
| 0007 | testcontainers-go Docker E2E testing strategy |
| 0008 | Signal context propagation and daemon lifecycle |
| 0009 | Config-relative state directory |
