# Phonewave

**A file-based message courier daemon that routes D-Mails between AI agent tool repositories.**

Phonewave watches outbox directories via fsnotify, reads YAML frontmatter to determine the `kind` of each D-Mail, routes it to the correct inbox(es) based on an auto-derived routing table, and removes the source file after successful delivery. Failed deliveries are saved to an error queue with automatic retry.

```bash
phonewave init ./repo-a ./repo-b ./repo-c
phonewave run --verbose
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

Phonewave scans these manifests, derives routes, and ensures every produced D-Mail reaches its consumer(s).

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
| `phonewave doctor` | Verify ecosystem health (paths, endpoints, PID conflicts) |
| `phonewave run` | Start the courier daemon (foreground) |
| `phonewave status` | Show daemon state, uptime, and 24h delivery statistics |
| `phonewave --version` | Show version and exit |

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

### `run` command

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--verbose` | `-v` | `false` | Log all delivery events to stderr |
| `--dry-run` | `-n` | `false` | Detect events without delivering |
| `--retry-interval` | `-r` | `0` (disabled) | Error queue retry interval (e.g. `60s`, `5m`) |
| `--max-retries` | `-m` | `10` | Maximum retry attempts per failed D-Mail |

## Tracing (OpenTelemetry)

Phonewave instruments daemon operations with OpenTelemetry spans. Tracing is off by default (noop tracer, zero overhead) and activates when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

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
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 phonewave run --verbose

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
phonewave run --verbose
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
just lint           # fmt check + vet + markdown lint
just lint-md        # Lint markdown files only
just check          # fmt + vet + test (pre-commit check)
just clean          # Clean build artifacts
just test-docker    # Docker lifecycle tests
just test-all       # All tests including Docker
just jaeger         # Start Jaeger trace viewer
just jaeger-down    # Stop Jaeger
just prek-install   # Install prek hooks
just prek-run       # Run all prek hooks
```

## File Structure

```
+-- cmd/phonewave/
|   +-- main.go              CLI entry point + subcommand routing
|   +-- main_test.go         CLI arg parsing tests
+-- phonewave.go             Init/Add/Remove/Sync orchestration (library)
+-- scanner.go               SKILL.md parser + endpoint discovery
+-- router.go                Route derivation engine (produces/consumes matching)
+-- config.go                phonewave.yaml read/write/merge
+-- daemon.go                fsnotify daemon + event loop + retry
+-- delivery.go              D-Mail delivery pipeline (atomic write)
+-- deliverylog.go           Append-only delivery log + error queue
+-- status.go                Daemon status + 24h statistics
+-- doctor.go                Ecosystem health checker
+-- telemetry.go             OpenTelemetry tracer setup (noop default)
+-- logger.go                Colored logging
+-- init.go                  Multi-repo init flow
+-- *_test.go                Tests
+-- justfile                 Task runner
+-- docker/
|   +-- compose.yaml         Jaeger v2 for trace viewing
|   +-- jaeger-v2-config.yaml
+-- docs/
|   +-- phonewave-directory.md
+-- testdata/
    +-- Dockerfile.test      Docker lifecycle test image
```

## Prerequisites

- Go 1.24+
- [just](https://just.systems) for task automation
- [Docker](https://www.docker.com/) for tracing (Jaeger) and lifecycle tests

## License

Apache License 2.0
See [LICENSE](./LICENSE) for details.
