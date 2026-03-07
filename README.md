# Phonewave

**A D-Mail courier daemon that watches outboxes, routes messages to matching inboxes, and retries failed deliveries across tool repositories.**

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
| **Error Queue** | `.phonewave/.run/error_queue.db` | Failed D-Mails waiting for retry (SQLite, like unsent D-Mails) |

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
          |  .run/error_queue.db |
          +----------------------+
```

## Scope

**What Phonewave does:**
- Watch outbox directories and route D-Mails by `kind` to matching inboxes
- Derive routing tables from SKILL.md manifests automatically
- Retry failed deliveries with exponential backoff (at-least-once delivery)
- Track all deliveries in an append-only log

**What Phonewave does NOT do:**
- Transform or inspect message content (routes as-is)
- Execute tools or manage tool lifecycles
- Guarantee exactly-once delivery (uses at-least-once + idempotent receivers)
- Store configuration in databases (uses `phonewave.yaml` only)

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
just --list         # Show all available tasks
just check          # Pre-commit: fmt + vet + test
just install        # Build and install to /usr/local/bin
just semgrep        # Run layer enforcement rules
just test-e2e       # Docker E2E tests
just jaeger         # Start Jaeger trace viewer
```

See `justfile` for the full task list.

## Project Layout

```
cmd/phonewave/          CLI entry point
internal/
  cmd/                  Cobra commands (init, run, doctor, status, etc.)
  usecase/              Business logic (PolicyEngine + handlers)
  session/              I/O orchestration (scanner, router, delivery)
  eventsource/          Event persistence (JSONL append-only)
  domain/               Pure domain types
  platform/             Platform adapters (OTel, logger)
docs/                   Documentation, ADRs, CLI reference
tests/                  Scenario (L1-L4) and Docker E2E tests
.semgrep/               Layer enforcement rules
```

For detailed structure, see [docs/conformance.md](docs/conformance.md).

## What / Why / How

See [docs/conformance.md](docs/conformance.md) for the full conformance table (single source).

## Documentation

- [docs/](docs/README.md) — Full documentation index
- [docs/conformance.md](docs/conformance.md) — What/Why/How conformance table
- [docs/phonewave-directory.md](docs/phonewave-directory.md) — `.phonewave/` directory structure
- [docs/policies.md](docs/policies.md) — Event → Policy mapping
- [docs/otel-backends.md](docs/otel-backends.md) — OTel backend configuration
- [docs/adr/](docs/adr/README.md) — Architecture Decision Records

## Prerequisites

- Go 1.26+
- [just](https://just.systems) for task automation
- [Docker](https://www.docker.com/) for tracing (Jaeger) and lifecycle tests
- [uv](https://docs.astral.sh/uv/) (optional) for SKILL.md spec validation via skills-ref submodule

## License

Apache License 2.0
See [LICENSE](./LICENSE) for details.
