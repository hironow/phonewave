# phonewave

## Workflow

- Do NOT use git worktrees (`EnterWorktree`, `isolation: "worktree"`). Work directly on the current branch.

## Repository Structure

- Entry: `cmd/phonewave/main.go` (signal.NotifyContext + ExitCode)
- CLI: `internal/cmd/` (cobra v1.10.2, `NewRootCommand()` exported for testability)
- Library: root package `phonewave` (daemon, delivery, config types, router, event, command, policy, logger, telemetry, metrics)
- Session: `internal/session/` (config I/O, scanner, init/add/remove/sync, doctor, status, validate, daemon setup)
- Eventsource: `internal/eventsource/` (event store + lifecycle; flat `.jsonl` storage, `os.Remove` pruning)
- OTel: `internal/cmd/telemetry.go` (initTracer + OTLP HTTP exporter), `telemetry.go` (noop default)
- Docker: `docker/compose.yaml` + `docker/jaeger-v2-config.yaml` (Jaeger v2)
- Docker E2E: `docker/compose-e2e.yaml` (testcontainers-go lifecycle tests)
- Semgrep: `.semgrep/cobra.yaml` (canonical source — copy to other 3 tools)
- Release: `.goreleaser.yaml` (darwin/linux x amd64/arm64)

## CLI Design

- `cobra.EnableTraverseRunHooks = true` in `init()` (not constructor)
- All commands use `RunE` (not `Run`)
- `--verbose`, `--config` are PersistentFlags on root
- Default subcommand: `phonewave [flags]` → prepends `run` via `NeedsDefaultRun`
- OTel tracer shutdown: `PersistentPreRunE` + `cobra.OnFinalize` + `sync.Once`
- State directory: derived from config path via `configBase(cmd)`

## Test Layout

- Unit tests: `*_test.go` colocated with source (Go convention)
    - Root tests use `package phonewave` (in-package) — daemon/delivery internals require direct access
    - `lifecycle_test.go` uses `package phonewave_test` (external) — imports both root and `internal/session`
    - `cmd/phonewave/main_test.go` uses `package main` for CLI arg parsing tests
- E2E tests: `tests/e2e/*_test.go` with `//go:build e2e` tag, `package e2e` (testcontainers-go)
    - `cli_docker_test.go` — CLI subcommand tests in container
    - `daemon_docker_test.go` — daemon behaviour (retry, error queue, burst)
    - `lifecycle_docker_test.go` — single-container lifecycle
    - `otel_docker_test.go` — OTel tracing with Jaeger container
    - `compose-e2e.yaml` + `Dockerfile.e2e` — Docker compose for E2E
- Race tests: `race_test.go` colocated with source (run with `just test-race`)

## ADR (Architecture Decision Records)

- `docs/adr/` — phonewave is **canonical source** for shared ADRs (0000-0005)
- Shared ADRs apply to all 4 tools (phonewave, sightjack, paintress, amadeus)
- Tool-specific ADRs: 0006+ (goreleaser, testcontainers, signal propagation, config-relative state)
- Changes to shared ADRs require cross-tool review via Linear

## D-Mail Protocol

- phonewave validates `dmail-schema-version: "1"` on all D-Mail files
- Valid kinds: `specification`, `report`, `feedback`, `convergence`
- Kind validation: `delivery.go` (`ValidateKind`) + `internal/session/scanner.go` (SKILL.md parsing)
- SKILL.md capabilities must be under `metadata` with `dmail-schema-version` — top-level `produces`/`consumes` is rejected

## Build & Test

```bash
just build          # build with version from git tags
just install        # build + install to /usr/local/bin
just test           # all tests, 300s timeout
just test-race      # with race detector
just test-e2e       # Docker E2E tests
just test-all       # test + test-e2e
just check          # fmt + vet + test
just semgrep        # cobra semgrep rules
just lint           # vet + markdown lint + gofmt check
```
