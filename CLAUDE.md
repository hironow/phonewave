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
