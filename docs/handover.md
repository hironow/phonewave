# Handover

**Last updated:** 2026-06-10 (JST)
**Updated by:** claude (AI draft from git history — review before trusting)

## Current State

Phonewave is the courier daemon of the D-Mail protocol ecosystem: it scans
repositories for tool endpoints, derives a routing table from SKILL.md
manifests, watches outboxes, delivers D-Mails atomically, and retries
failures via an SQLite error queue (README). Recent work on `main` finished
MCP pivot wording/telemetry docs (#174, #175), added prek markdown hooks
(#173), suppressed pre-existing lint findings (#169–#172), normalized
generated CLI docs (#168), hardened session close handling (#167), and
migrated e2e tests to testcontainers-go (#165). Last commit: `c2f920d`
"docs: add decision queue for human-review items (#178)" on 2026-06-10.

## In Progress

不明 (git 履歴からは判別できず) — no open feature branch is evident in the
shallow clone; recent commits are docs/lint/test hardening.

## Next Actions

1. requester による docs/intent.md ドラフトのレビューと確定
2. Work through the human-review items in `docs/decision-queue.md` (added 2026-06-10, #178)

## Known Risks / Blockers

- `docs/intent.md` / `docs/handover.md` were deliberately gitignored in #163; this PR adds them with `git add -f`. Decide whether to track them or keep them local-only.

## Context the Next Actor Needs

- Task runner is `just`; `just check` runs fmt + vet + golangci-lint + semgrep + root-guard + tests + docs-check
- Project-specific semgrep rules live under `.semgrep/`; pre-commit hooks via `.pre-commit-config.yaml` (prek); toolchain pinned in `mise.toml`
- Naming comes from Steins;Gate (Phone Microwave / D-Mail) — see README concept table before touching domain terms
- Runtime state lives under `.phonewave/`: `.run/error_queue.db` (SQLite retry queue) and `insights/` (git-tracked failure-knowledge ledger)
- D-Mail Schema v1 kinds and the SKILL.md `metadata` declaration format are load-bearing — schema assets are under `schema/`, sample data under `testdata/`
- Releases via GoReleaser; e2e tests use testcontainers-go

## Relevant Files and Commands

- `README.md` — courier flow, D-Mail Schema v1 kinds, SKILL.md manifest format
- `docs/decision-queue.md` — open human-review items
- `docs/dmail-protocol-conventions.md` and `docs/phonewave-directory.md` — protocol and state-dir conventions
- `schema/` — D-Mail schema assets; `testdata/` — fixtures
- `justfile` — `just check` (full gate), `just test`, `just lint`, `just semgrep`
- `cmd/` and `internal/` — CLI entrypoints and core implementation
