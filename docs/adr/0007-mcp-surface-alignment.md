# 0007. MCP surface alignment (dot-free names, instructions, delivery stats, telemetry label)

**Date:** 2026-06-10
**Status:** Accepted

## Context

After the write-path restoration wave (shared ADR S0045), phonewave was
the only tool with dotted MCP tool names (`phonewave.ping` — the
canonical Claude Code form has no documented dot normalization,
conformance constraint C1) and without `instructions` in the
initialize handshake (C6). Separately, the telemetry status tag
`"deprecated"` on the visibility tools — a cost-monitoring marker from
the jun15 billing split (PR #147) — kept being misread as
"scheduled for removal" (it caused a wrong hygiene work item in refs
issue 0034's own genesis). And the most common session question — "did
my d-mail get delivered?" — was only answerable via the CLI.

## Decision

1. **Dot-free tool names**: `phonewave.ping` → `ping`,
   `phonewave.outbox_status` → `outbox_status`,
   `phonewave.inbox_status` → `inbox_status`. No aliases: entry-skill
   invocations were zero when renamed (same justification as the
   sibling renames in S0045's wave).
2. **`instructions` in the initialize handshake**, stating the courier
   role and that `phonewave run` must not be started from a session.
3. **`outbox_status` carries 24h delivery stats** (`delivered_24h` /
   `failed_24h` / `retried_24h`) via the existing
   `ParseDeliveryStats` reader.
4. **Telemetry status label `"deprecated"` → `"visibility"`**. The
   semantics are unchanged (a marker distinguishing visibility-tool
   invocations in `mcp.tool.invocations`), but this IS a label-
   cardinality change for time series: dashboards reading
   `result.status="deprecated"` must switch to `"visibility"`; the
   pre-rename series ends on 2026-06-10.

## Consequences

### Positive

- Full C1/C6 conformance across all 5 tools; the misleading label is
  gone; sessions can gate d-mail workflows on actual delivery results.

### Negative

- Telemetry series break at the rename date (documented here; local
  Jaeger/W&B usage keeps the blast radius small).

### Neutral

- The marker's purpose (post-6/15 cost monitoring) continues under the
  new name.
