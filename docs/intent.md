# Intent

**Last updated:** 2026-06-10
**Requester:** hironow
**Status:** DRAFT — AI が README / git 履歴から起草。requester 未確認
**Work unit:** phonewave — D-Mail courier daemon for the D-Mail protocol ecosystem

## Goal

Provide a courier daemon that watches outbox directories via fsnotify, reads
each D-Mail's YAML frontmatter `kind`, routes it to the matching inbox(es)
based on a routing table auto-derived from SKILL.md produces/consumes
manifests, delivers via atomic write, and queues failed deliveries for
automatic retry.

## Success Criteria

- `just check` passes (fmt, vet, golangci-lint, semgrep, root-guard, tests, docs-check) — quality gate defined in the justfile and wired into CI under `.github/`
- The end-to-end courier flow described in README works: `phonewave init <repos...>` derives routes from SKILL.md manifests, `phonewave run` watches outboxes and delivers to inboxes (covered by e2e tests incl. the MCP tools-list handshake test, migrated to testcontainers-go)
- Product-level success criteria beyond these mechanical gates: 未定義 — Open Questions 参照

## Scope

### In scope

- Endpoint scanning (`.siren/`, `.expedition/`, `.gate/`, ...), SKILL.md manifest parsing, and routing-table derivation for D-Mail Schema v1 kinds
- Outbox watching (fsnotify), atomic delivery (temp + rename), delivery logging, error queue with retry (`.phonewave/.run/error_queue.db`), and the insight ledger (`.phonewave/insights/`)

### Out of scope (Non-goals)

- Producing or consuming D-Mail content itself — phonewave is the Courier/Coordinator with no own endpoint; sightjack/paintress/amadeus own the producing/consuming roles (per README role table)

## Constraints

- Go module; lint/semgrep/test gates enforced via justfile recipes and pre-commit hooks (`.golangci.yaml`, `.semgrep/`, `.pre-commit-config.yaml`)
- D-Mail Schema v1 and Agent Skills v1 SKILL.md format (capabilities declared under `metadata` with `dmail-schema-version: "1"`) — schema assets live under `schema/`
- Released via GoReleaser (`.goreleaser.yaml`) and distributed through the `hironow/homebrew-tap` cask

## Open Questions

- [ ] requester による本ドラフトのレビュー
- [ ] Product-level success criteria (delivery reliability/latency targets, retry policy guarantees) — not stated in README or docs
- [ ] Deadlines or milestone targets — none found in the repo
- [ ] `docs/intent.md` and `docs/handover.md` were deliberately gitignored in #163 — confirm whether tracking them via this PR is desired, or whether they should stay local-only
- [ ] Disposition of the items in `docs/decision-queue.md` (added 2026-06-10, #178) — which are still open?
