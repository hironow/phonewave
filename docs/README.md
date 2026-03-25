# phonewave docs

## Architecture

- [conformance.md](conformance.md) — What/Why/How conformance table (single source)
- [phonewave-directory.md](phonewave-directory.md) — `.phonewave/` directory structure specification
- [policies.md](policies.md) — Event → Policy mapping (WHEN event THEN command)
- [otel-backends.md](otel-backends.md) — OpenTelemetry backend configuration (Jaeger, Weave)
- [dmail-protocol-conventions.md](dmail-protocol-conventions.md) — D-Mail filename uniqueness and archive retention conventions
- [testing.md](testing.md) — Test strategy and conventions

## CLI Reference

- [phonewave](cli/phonewave.md) — Root command
- [phonewave init](cli/phonewave_init.md) — Scan repositories, discover tools, generate routing table
- [phonewave run](cli/phonewave_run.md) — Start the courier daemon
- [phonewave add](cli/phonewave_add.md) — Add a new repository to the ecosystem
- [phonewave remove](cli/phonewave_remove.md) — Remove a repository from the ecosystem
- [phonewave sync](cli/phonewave_sync.md) — Re-scan all repositories, reconcile routing table
- [phonewave status](cli/phonewave_status.md) — Show daemon and delivery status
- [phonewave doctor](cli/phonewave_doctor.md) — Verify ecosystem health
- [phonewave clean](cli/phonewave_clean.md) — Remove runtime state from .phonewave/
- [phonewave archive-prune](cli/phonewave_archive-prune.md) — Prune expired event files
- [phonewave version](cli/phonewave_version.md) — Print version, commit, and build information
- [phonewave update](cli/phonewave_update.md) — Self-update phonewave to the latest release

## Architecture Decision Records

- [adr/](adr/README.md) — Tool-specific ADRs
- [shared-adr/](shared-adr/README.md) — Cross-tool shared ADRs (S0001–S0034)
