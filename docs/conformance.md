# What / Why / How Conformance

This is the single source of truth for phonewave's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | File-based message courier daemon that routes D-Mails between AI agent tool repositories |
| **Why** | Enable inter-tool communication without shared databases or direct API coupling |
| **How** | fsnotify watch on outbox directories → YAML frontmatter routing → atomic inbox delivery → SQLite error queue with retry |
| **Input** | D-Mail `.md` files written to outbox directories by other tools |
| **Output** | D-Mail `.md` files delivered to inbox directories of consuming tools |
| **Telemetry** | OTel spans: `phonewave.run`, `startup_scan`, `handle_event`, `deliver_data` |
| **External Systems** | File system (fsnotify), OTel exporter (Jaeger/Weave) |

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
