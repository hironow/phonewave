# phonewave docs

## Architecture

- [phonewave-directory.md](phonewave-directory.md) — `.phonewave/` directory structure specification
- [policies.md](policies.md) — Event → Policy mapping (WHEN event THEN command)
- [otel-backends.md](otel-backends.md) — OpenTelemetry backend configuration (Jaeger, Weave)
- [testing.md](testing.md) — Test strategy and conventions

## CLI Reference

- [phonewave](cli/phonewave.md) — Root command
- [phonewave init](cli/phonewave_init.md) — Initialize routing for repositories
- [phonewave run](cli/phonewave_run.md) — Start the courier daemon
- [phonewave add](cli/phonewave_add.md) — Add a repository to the routing table
- [phonewave remove](cli/phonewave_remove.md) — Remove a repository
- [phonewave sync](cli/phonewave_sync.md) — Sync routing table
- [phonewave status](cli/phonewave_status.md) — Show daemon status
- [phonewave doctor](cli/phonewave_doctor.md) — Diagnose configuration issues
- [phonewave clean](cli/phonewave_clean.md) — Clean state files
- [phonewave archive-prune](cli/phonewave_archive-prune.md) — Prune archived D-Mails
- [phonewave version](cli/phonewave_version.md) — Show version
- [phonewave update](cli/phonewave_update.md) — Self-update

## Architecture Decision Records

See [adr/README.md](adr/README.md) for the full index.
