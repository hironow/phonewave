# Handover

**Last updated:** 2026-05-22 (asia/tokyo, 0028 semgrep symmetric rule landed)
**Updated by:** Claude Opus 4.7 session

## Current State

jun15 MCP pivot (refs/issues/0027) **全 phase 完了 + archive 入り**、
**かつ 0028 cross-tool semgrep gate 強化も完了**。
phonewave は LLM 非使用ツールだが、 cross-tool visibility のため
optional MCP server を expose する pattern を Phase 2d で確立、 Phase 4
#2 で dead-letter / oldest-age telemetry を追加。

phonewave 固有の jun15 landmark:

- ADR 0006 (= `docs/adr/0006-mcp-pivot.md`) で architectural pin 固定
- 3 MCP tool 全 real impl (= ping / outbox_status / inbox_status)
- 0028 (PR #151 `a27a0e3`): cross-tool symmetric regression prevention。
  新 semgrep rule `jun15-no-print-flag-literal-go` を 5 ツール全てに
  追加 (= dynamic args spread を catch する regex rule)。 phonewave は
  元から LLM 非使用なので **production code 変更なし、 future regression
  防止のみ** (= 誤って headless LLM 経路が混入するのを gate で防ぐ)
- Phase 4 #2 (PR #149 `56e0b82`): `phonewave.outbox_status` /
  `inbox_status` response に下記 2 field 追加:
  - `dead_letter_count`: SQLite delivery store (`.run/delivery.db`)
    の `DeadLetterCount()` を呼ぶ (db 不在 → 0)
  - `oldest_age_seconds`: outbox / inbox dir 配下 file の最古 mtime
    からの経過秒数 (空 → 0)
- jun15 launch 前後の deadline 観察 (= dead-letter queue overflow /
  D-Mail 滞留検出) に必要な指標を MCP 経由で session に提供
- D-Mail consume には繋がない (= human-initiated constraint 維持)
- `.semgrep/jun15-no-headless-llm.yaml` **6 rule** (= base 5 + 0028 で
  `jun15-no-print-flag-literal-go` 追加) は phonewave にも permanent
  block で適用

## In Progress

なし。 jun15 MCP pivot に関する作業は完了し refs 0027 は archive (=
`tap/refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`)。

## Next Actions

なし (= Phase 4 #1-#4 全完了)。 後続作業候補は別 issue で fork:

1. Phase 3 cost (c) Anthropic dashboard credit 0 verify (= 2026-06-15
   launch 以降の operational evidence)

## Known Risks / Blockers

- phonewave は元々 LLM 非使用なので `claude --print` invocation 削除
  は対象外、 daemon role (= D-Mail courier) は維持
- `oldest_age_seconds` 計算は dir scan ベースなので large outbox では
  O(N) コスト。 現状の D-Mail throughput では問題ない

## Context the Next Actor Needs

- **canonical plan archive**: `tap/refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`
- **post-mortem**: `tap/refs/HTMLification/lessons/0027-jun15-mcp-pivot-post-mortem.html`
- **billing boundary 原則**: LLM 発火は常に human-initiated、 daemon は route まで
- **D-Mail 9-field envelope schema**: cross-tool contract base、 phonewave
  は schema validation せず opaque file relay role に徹する

## Relevant Files and Commands

- `docs/adr/0006-mcp-pivot.md` - architectural pin
- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (6 rule、 0028 で dynamic args spread catch rule 追加)
- `internal/session/mcp_server.go` - phonewave MCP server (3 tool real impl + dead-letter telemetry)
- `internal/session/delivery_store.go` - SQLite delivery store (= `.run/delivery.db`)
- `internal/cmd/mcp.go` - `phonewave mcp` cobra subcommand
- `just lint` - golangci-lint v2 + markdownlint (0 issues 維持)
- `just semgrep` - semgrep gate (0 findings 維持)
- `go test -count=1 ./...` - phonewave test suite
