# phonewave

## Workflow

- Do NOT use git worktrees (`EnterWorktree`, `isolation: "worktree"`). Work directly on the current branch.

## Repository Structure

- Entry: `cmd/phonewave/main.go` (signal.NotifyContext + InitTracer defer + ExitCode)
- CLI: `internal/cmd/` (cobra v1.10.2, `NewRootCommand()` exported for testability)
- Library: root package `phonewave` (daemon, delivery, config, scanner, router, doctor, status, telemetry, logger)
- OTel: `telemetry.go` (noop default + OTLP HTTP exporter)
- Docker: `docker/compose.yaml` + `docker/jaeger-v2-config.yaml` (Jaeger v2)
- Docker E2E: `docker/compose-e2e.yaml` (testcontainers-go lifecycle tests)
- Semgrep: `.semgrep/cobra.yaml` (canonical source — copy to other 3 tools)
- Release: `.goreleaser.yaml` (darwin/linux x amd64/arm64)

## CLI Design

- `cobra.EnableTraverseRunHooks = true` in `init()` (not constructor)
- All commands use `RunE` (not `Run`)
- `--verbose`, `--config` are PersistentFlags on root
- Tracer lifecycle: `main.go` defer (NOT cobra hooks — PersistentPostRunE skipped on error)
- State directory: derived from config path via `configBase(cmd)`

## Test Layout

- Unit tests: `*_test.go` colocated with source (Go convention)
  - All root tests use `package phonewave` (in-package) — daemon/delivery internals require direct access
  - `cmd/phonewave/main_test.go` uses `package main` for CLI arg parsing tests
- Docker E2E: `*_docker_test.go` with `//go:build docker` tag (testcontainers-go)
  - `cli_docker_test.go` — CLI subcommand tests in container
  - `daemon_docker_test.go` — daemon behaviour (retry, error queue, burst)
  - `lifecycle_docker_test.go` — single-container lifecycle
  - `lifecycle_multicontainer_test.go` — cross-container D-Mail delivery
  - `otel_docker_test.go` — OTel tracing with Jaeger container
- No `tests/` directory — all tests colocated with source per Go convention

## ADR (Architecture Decision Records)

- `docs/adr/` — phonewave is **canonical source** for shared ADRs (0000-0005)
- Shared ADRs apply to all 4 tools (phonewave, sightjack, paintress, amadeus)
- Tool-specific ADRs: 0006+ (goreleaser, testcontainers, signal propagation, config-relative state)
- Changes to shared ADRs require cross-tool review via Linear

## D-Mail Protocol

- phonewave validates `dmail-schema-version: "1"` on all D-Mail files
- Valid kinds: `specification`, `report`, `feedback`, `convergence`
- Kind validation: `delivery.go` (`ValidateKind`) + `scanner.go` (SKILL.md parsing)
- SKILL.md capabilities must be under `metadata` with `dmail-schema-version` — top-level `produces`/`consumes` is rejected

## Build & Test

```bash
just build          # build with version from git tags
just install        # build + install to /usr/local/bin
just test           # all tests, 300s timeout
just test-race      # with race detector
just test-docker    # Docker lifecycle tests (requires Docker)
just test-all       # test + test-docker
just check          # fmt + vet + test
just semgrep        # cobra semgrep rules
just lint           # vet + markdown lint + gofmt check
```
