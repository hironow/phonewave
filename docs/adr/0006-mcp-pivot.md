# 0006. MCP pivot: phonewave optional visibility MCP server

**Date:** 2026-05-21
**Status:** Accepted

## Context

Starting 2026-06-15, Anthropic Claude Code subscription plans (Pro,
Max 5x, Max 20x) bill `claude -p` and Agent SDK usage against a
separate monthly Agent SDK credit pool ($20 / $100 / $200) that is
disjoint from the interactive usage quota.

paintress (ADR 0017), sightjack (ADR 0018), amadeus (ADR 0026), and
dominator (ADR 0003) each performed an LLM owner inversion in Phase
1-2c (refs/issues/0027): the human-initiated claude code interactive
session became the LLM owner, and each Go CLI was reduced to an MCP
server exposing its data plane.

phonewave is the **courier daemon** that routes D-Mail messages
between the four tools' outbox / inbox directories. It does NOT
invoke `claude -p`, import the Anthropic Agent SDK, or otherwise
participate in the LLM ownership question — its production code
path is purely file-system + OTel + SQLite outbox bookkeeping.

The refs/issues/0027 §4 table marked phonewave's MCP work as
`optional: phonewave.outbox_status MCP (= visibility)`. Phase 2d
implements this optional visibility surface so the five-tool MCP
pattern is symmetric (= every tool now has a `<tool> mcp` subcommand
exposing its data plane) and a claude code session can read
phonewave queue depths without shelling out to `phonewave status`
/ `phonewave metrics`.

## Decision

phonewave adds a purely additive MCP server. From this commit
forward, the architecture is:

1. **`phonewave mcp` MCP server.** A new cobra subcommand starts a
   stdio JSON-RPC 2.0 MCP server. Phase 2d MVP exposes
   `phonewave.ping` plus two visibility stubs: `phonewave.outbox_status`
   (tool → outbox depth + dead-letter count + oldest age) and
   `phonewave.inbox_status` (tool → inbox depth + seen / ack counts).
   Real wiring against the courier daemon state lands in a follow-up
   commit; the stubs return contract descriptors so claude code
   clients can exercise the interface end-to-end immediately.
2. **No deprecated LLM invocation path to redirect.** Unlike the
   other four tools, phonewave never invoked `claude -p`, so this
   MCP server does NOT replace anything — it is a brand-new
   visibility surface.
3. **No D-Mail envelope decode.** phonewave routes messages by
   filename + metadata only; it does not parse the envelope body.
   So unlike paintress / sightjack / amadeus / dominator, phonewave
   does NOT carry an `internal/domain/dmail_envelope.go`. This is a
   natural reflection of phonewave's body-agnostic courier
   responsibility.
4. **The semgrep gate is preventive, not transitional.** The same
   five `jun15-no-headless-llm` rules apply repo-wide with zero
   findings; no transitional excludes are needed because phonewave
   had no `claude --print` calls to begin with.

The five-tool MCP surface is now:

| Tool      | MCP server         | Role                                  |
|-----------|--------------------|---------------------------------------|
| paintress | `paintress mcp`    | data plane: journal / gradient / Linear |
| sightjack | `sightjack mcp`    | data plane: scan / wave / cluster     |
| amadeus   | `amadeus mcp`      | data plane: review / convergence / PR |
| dominator | `dominator mcp`    | data plane: NFR config / k6 history   |
| phonewave | `phonewave mcp`    | visibility: outbox / inbox queue depth |

The claude code session can attach any subset of the five via
`--mcp-config`. The session's reasoning then orchestrates the five
tools through their slash command skills (e.g. `/expedition-next`,
`/sightjack-scan`, `/review-gate`, `/nfr-judge`).

## Enforcement inventory

### Entry points

- `phonewave mcp` cobra subcommand (= the only MCP entry point;
  no production CLI path previously invoked LLM).

### Persistent data carried through the new path

- Outbox / inbox queue depth reads (= read-only, no mutation).
- Dead-letter queue depth reads.
- OTel span attributes (`messaging.*`) continue to flow when the
  MCP tools are wired in a follow-up commit.

### Bypass candidates

- Direct `exec.Command("claude", "--print", ...)` from Go code —
  blocked by `jun15-no-claude-print-exec-go` (= preventive, no
  existing violation).
- Shell wrappers, Anthropic SDK imports, `ANTHROPIC_API_KEY` reads
  — all blocked by the same five rules.
- Hooks streaming inbox content on stdout — handled at session-level
  documentation; phonewave itself never emits to stdout in daemon
  mode.

### Tests proving coverage

- `internal/session/mcp_server_test.go` — five tests prove the
  `phonewave mcp` stdio server advertises all three Phase 2d tools
  (`phonewave.ping` + `outbox_status` + `inbox_status`), dispatches
  each correctly, and returns the JSON-RPC `-32601` error for
  unknown tools / methods.
- `just semgrep` — 74 rules, 0 findings, including the five
  `jun15-no-headless-llm` gate rules (preventive only).

## Consequences

### Positive

- Five-tool MCP surface is now symmetric. A claude code session can
  see queue depths via MCP tools rather than parsing CLI output, so
  workflows like `/expedition-next` can query `phonewave.inbox_status`
  to know whether paintress has unread D-Mail before acting.
- The semgrep gate is repo-wide enforced from day one; future
  contributors cannot introduce headless LLM calls into phonewave
  without explicit ADR amendment.
- Zero behavior change to existing phonewave subcommands (`run`,
  `status`, `metrics`, etc.) — this commit is purely additive.

### Negative

- The `outbox_status` / `inbox_status` stubs return contract
  descriptors only. Until the real wiring lands, claude code
  sessions cannot rely on accurate depth data.
- One more subcommand to maintain on top of the existing 13.

### Neutral

- `internal/domain/dmail_envelope.go` is intentionally absent from
  phonewave (= different from paintress / sightjack / amadeus /
  dominator). This is captured in the §Decision §3 note and reflects
  phonewave's body-agnostic courier responsibility.
- No tool-specific skill (`plugins/phonewave/skills/*`) is shipped
  in Phase 2d because phonewave's visibility tools are intended to
  be called from other tools' skills (e.g., `/expedition-next` calls
  `phonewave.inbox_status` to gate its own actions). A standalone
  `/phonewave-status` skill may land in a follow-up commit if user
  demand emerges.

## References

- refs/issues/0027 — canonical plan including all four codex review
  rounds, the billing boundary table, the mechanical gate, the MVP
  scope reduction, and the §4 phonewave row marked `optional:
  visibility`.
- paintress ADR 0017 — canonical 9-commit pattern that originated
  the LLM owner inversion.
- sightjack ADR 0018 — Phase 2a confirmation on the scan / wave
  pipeline.
- amadeus ADR 0026 — Phase 2b confirmation on the review / convergence
  pipeline.
- dominator ADR 0003 — Phase 2c confirmation on the NFR judge /
  k6 pipeline.
- Local ADRs 0001-0005 — the goreleaser / testcontainers / signal /
  config / actor-type architectural decisions this ADR preserves.
- <https://code.claude.com/docs/en/headless> — 2026-06-15 credit
  pool change announcement.
- <https://support.claude.com/en/articles/15036540> — per-plan
  credit allocation table.
