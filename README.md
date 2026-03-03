# Phonewave

**A file-based message courier daemon that routes D-Mails between AI agent tool repositories.**

Phonewave watches outbox directories via fsnotify, reads YAML frontmatter to determine the `kind` of each D-Mail, routes it to the correct inbox(es) based on an auto-derived routing table, and removes the source file after successful delivery. Failed deliveries are saved to an error queue with automatic retry.

```bash
phonewave init ./repo-a ./repo-b ./repo-c
phonewave run -v
```

These two commands make Phonewave:

1. Scan repositories for tool endpoints (`.siren/`, `.expedition/`, `.gate/`, etc.)
2. Parse `SKILL.md` manifests to discover produces/consumes relationships
3. Derive a routing table matching `kind` producers to consumers
4. Watch all outbox directories for new `.md` files
5. Deliver each D-Mail to the correct inbox(es) via atomic write (temp + rename)
6. Log every delivery, archive removals, and queue failures for retry

## Why "Phonewave"?

The name comes from [Steins;Gate](https://en.wikipedia.org/wiki/Steins;Gate), where the "Phone Microwave (name subject to change)" — or Phonewave — is a modified microwave oven that can send text messages to the past (D-Mails). In the show, D-Mails are short messages that change the timeline when delivered.

This maps to the courier daemon's design:

| Steins;Gate | Phonewave | Design Meaning |
|---|---|---|
| **Phonewave** | This binary | The device that sends D-Mails |
| **D-Mail** | `.md` file with YAML frontmatter | A message routed by `kind` |
| **Worldline** | Repository state | Each delivery changes the target repo's state |
| **Divergence Meter** | Delivery log | Tracks what was delivered, when, where |
| **Error Queue** | `.phonewave/errors/` | Failed D-Mails waiting for retry (like unsent D-Mails) |

## D-Mail Protocol

Phonewave is the courier layer for the D-Mail protocol. Four tools participate in the ecosystem:

| Tool | Role | Endpoint |
|------|------|----------|
| **sightjack** | Designer / Protocol spec owner | `.siren/` |
| **paintress** | Implementer | `.expedition/` |
| **amadeus** | Verifier | `.gate/` |
| **phonewave** | Courier / Coordinator | (no endpoint — routes between others) |

Each tool declares its D-Mail capabilities in `SKILL.md` manifests:

- `skills/dmail-sendable/SKILL.md` — declares what `kind`s the tool produces (writes to `outbox/`)
- `skills/dmail-readable/SKILL.md` — declares what `kind`s the tool consumes (reads from `inbox/`)

D-Mail Schema v1 defines four message kinds:

| Kind | Flow | Description |
|------|------|-------------|
| `specification` | sightjack → paintress | Issue specification ready for implementation |
| `report` | paintress → amadeus | Implementation report for verification |
| `feedback` | amadeus → sightjack, paintress | Corrective feedback from verifier |
| `convergence` | amadeus → sightjack | World line convergence alert |

SKILL.md uses Agent Skills v1 format with D-Mail declarations nested under `metadata`:

```yaml
---
name: dmail-sendable
description: Produces D-Mail messages to outbox for phonewave delivery.
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
      description: Issue specification ready for implementation
---
```

Phonewave scans these manifests, validates kind values, derives routes, and ensures every produced D-Mail reaches its consumer(s). D-Mail capabilities must be declared under `metadata` with `dmail-schema-version: "1"`.

## Architecture

```
Repository A                   Repository B
+-- .siren/                    +-- .expedition/
|   +-- outbox/  ----+         |   +-- inbox/  <----+
|   +-- inbox/  <-+  |         |   +-- outbox/ --+  |
|   +-- skills/   |  |         |   +-- skills/   |  |
+-- .gate/        |  |         +-- .gate/        |  |
    +-- inbox/ <--+--+--+          +-- inbox/ <--+  |
    +-- outbox/ --+  |  |          +-- outbox/ -----+
                     |  |
          phonewave  |  |
          +----------+--+--------+
          |                      |
          |  SKILL.md parser     |
          |       |              |
          |  Route derivation    |
          |       |              |
          |  phonewave.yaml      |
          |       |              |
          |  fsnotify daemon     |
          |       |              |
          |  Delivery pipeline   |
          |       |              |
          |  delivery.log        |
          |  .phonewave/errors/  |
          +----------------------+
```

## Subcommands

| Command | Description |
|---------|-------------|
| `phonewave init <repo...>` | Scan repositories, discover endpoints, derive routes, generate `phonewave.yaml` |
| `phonewave add <repo>` | Add a new repository to the ecosystem |
| `phonewave remove <repo>` | Remove a repository from the ecosystem |
| `phonewave sync` | Re-scan all repositories, reconcile routing table |
| `phonewave doctor` | Verify ecosystem health (paths, endpoints, SKILL.md spec compliance, PID conflicts) |
| `phonewave run` | Start the courier daemon (foreground) |
| `phonewave status` | Show daemon state, uptime, and 24h delivery statistics |
| `phonewave clean` | Remove runtime state from `.phonewave/` |
| `phonewave archive-prune` | Prune old archived D-Mail files |
| `phonewave version` | Print build version information |
| `phonewave update` | Update phonewave to the latest version |

## Usage

```bash
# Initialize with multiple repositories
phonewave init ./sightjack-repo ./paintress-repo ./amadeus-repo

# Check ecosystem health
phonewave doctor

# Start daemon (foreground, verbose)
phonewave run -v

# Dry run (detect events, don't deliver)
phonewave run -n

# With retry interval (check error queue every 120s)
phonewave run -r 120s

# With tracing enabled
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 phonewave run -v

# Check daemon status
phonewave status

# Add a new repo after initial setup
phonewave add ./new-repo

# Re-scan after endpoint changes
phonewave sync
```

## Options

### Global flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--verbose` | `-v` | `false` | Log all delivery events to stderr |
| `--config` | `-c` | `./phonewave.yaml` | Path to phonewave config file |
| `--output` | `-o` | `text` | Output format: `text` or `json` |

### `run` command

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dry-run` | `-n` | `false` | Detect events without delivering |
| `--retry-interval` | `-r` | `60s` | Error queue retry interval (0 to disable) |
| `--max-retries` | `-m` | `10` | Maximum retry attempts per failed D-Mail |

### `version` command

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--json` | `-j` | `false` | Output version info as JSON |

### `update` command

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--check` | `-C` | `false` | Check for updates without installing |

## Tracing (OpenTelemetry)

Phonewave instruments daemon operations with OpenTelemetry spans. Tracing is off by default (noop tracer, zero overhead) and activates when `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is set.

Each daemon operation creates an **independent root span** — there is no long-lived parent span covering the daemon lifetime. This follows OTel best practices for daemons.

```
daemon.startup_scan (root, per outbox dir)
+-- delivery.deliver (per file)

daemon.handle_event (root, per fsnotify event)
+-- delivery.deliver (per file)

daemon.retry_pending (root, per ticker fire)
+-- delivery.deliver (per retry)
```

```bash
# Start Jaeger (trace viewer)
just jaeger

# Run with tracing
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 phonewave run -v

# View traces at http://localhost:16686

# Stop Jaeger
just jaeger-down
```

## Setup

```bash
# Build and install
just install

# Initialize
phonewave init /path/to/repo-a /path/to/repo-b

# Check health
phonewave doctor

# Run daemon
phonewave run -v
```

## Development

```bash
# Task runner (just)
just build          # Build binary
just install        # Build and install to /usr/local/bin
just test           # Run all tests
just test-v         # Verbose test output
just test-race      # Tests with race detector
just cover          # Coverage report
just cover-html     # Open coverage in browser
just fmt            # Format code (gofmt)
just vet            # Run go vet
just semgrep        # Run semgrep rules (.semgrep/)
just lint           # fmt check + vet + markdown lint
just lint-md        # Lint markdown files only
just check          # fmt + vet + test (pre-commit check)
just clean          # Clean build artifacts
just test-e2e       # Docker E2E tests (build + run)
just test-e2e-shell # Interactive shell in E2E container
just test-e2e-down  # Clean up E2E containers
just test-cross-e2e # Cross-tool E2E tests (requires Docker)
just test-all       # All tests (unit + E2E)
just test-scenario-min  # L1 scenario test (minimal closed loop)
just test-scenario      # L1+L2 scenario tests (CI default)
just test-scenario-all  # All scenario tests (L1-L4)
just jaeger         # Start Jaeger trace viewer
just jaeger-down    # Stop Jaeger
just validate-skills <path> # Validate SKILL.md against Agent Skills spec
just docgen         # Generate CLI docs (Markdown)
just prek-install   # Install prek hooks
just prek-run       # Run all prek hooks
```

## File Structure

```
+-- cmd/phonewave/
|   +-- main.go                CLI entry point (signal handling, tracer init)
|   +-- main_test.go           CLI arg parsing + flag tests
+-- internal/cmd/
|   +-- root.go                Root cobra command + global flags
|   +-- run.go                 run subcommand + daemon startup
|   +-- init.go                init subcommand
|   +-- add.go                 add subcommand
|   +-- remove.go              remove subcommand
|   +-- sync.go                sync subcommand
|   +-- doctor.go              doctor subcommand
|   +-- status.go              status subcommand
|   +-- version.go             version subcommand (text/JSON output)
|   +-- update.go              update subcommand (self-update via GitHub)
|   +-- helpers.go             Shared CLI helpers (config path resolution)
+-- doc.go                      Package declaration (root-zero: all code in internal/)
+-- internal/usecase/           Use case layer (PolicyEngine + handlers)
+-- internal/session/           I/O orchestration layer
|   +-- init.go                Init/Add/Remove/Sync orchestration
|   +-- scanner.go             SKILL.md parser + endpoint discovery
|   +-- router.go              Route derivation engine
|   +-- status.go              Daemon status + 24h statistics
|   +-- doctor.go              Ecosystem health checker
|   +-- validate.go            skills-ref validation
+-- internal/eventsource/       Event store infrastructure (JSONL)
+-- internal/domain/            Pure domain functions
+-- internal/tools/docgen/      CLI doc generator
+-- tests/scenario/             Scenario tests (L1-L4, //go:build scenario)
+-- tests/e2e/                  Docker E2E tests (//go:build e2e)
+-- .semgrep/                   Semgrep rules (layer enforcement)
+-- .goreleaser.yaml            GoReleaser config (cross-platform)
+-- .github/workflows/          CI (test, vet, lint) + Release
+-- docker/                     Jaeger v2 for trace viewing
+-- skills-ref/                 Agent Skills reference validator (submodule)
+-- docs/
|   +-- adr/                   Architecture Decision Records
|   +-- cli/                   Auto-generated CLI reference
```

## Prerequisites

- Go 1.26+
- [just](https://just.systems) for task automation
- [Docker](https://www.docker.com/) for tracing (Jaeger) and lifecycle tests
- [uv](https://docs.astral.sh/uv/) (optional) for SKILL.md spec validation via skills-ref submodule

## License

Apache License 2.0
See [LICENSE](./LICENSE) for details.
