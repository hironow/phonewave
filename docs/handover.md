# Handover

**Last updated:** 2026-05-21 (asia/tokyo, Phase 2d kickoff)
**Updated by:** Claude Opus 4.7 session

## Current State

`feat/jun15-mcp-pivot` long-lived branch を切り、 refs/issues/0027
(jun15 MCP pivot v4) の Phase 2d (= phonewave optional visibility
MCP) を着手。 Phase 1-2c (paintress / sightjack / amadeus / dominator)
で確立した pattern を phonewave 用に adapt するが、 **phonewave は
courier daemon で LLM を一切使わない**ため、 deprecate stub / 既存
test 削除 / e2e t.Skip 等の cleanup work は不要。

本 commit (= scaffold) で配置済:

- `.semgrep/jun15-no-headless-llm.yaml`: 5 rule、 transitional
  exclude **なし** (= phonewave は元から `claude --print` 不使用
  なので preventive gate のみ)

## In Progress

- branch: `feat/jun15-mcp-pivot` (= long-lived feature branch、 main
  merge は Phase 2d 全完了後)
- linked issue: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- canonical pattern: paintress ADR 0017 + sightjack ADR 0018 +
  amadeus ADR 0026 + dominator ADR 0003 (= LLM owner inversion、
  Go CLI を MCP server data plane に縮約)
- Phase 2d MVP scope (= refs 0027 §4 phonewave row、 optional
  visibility):
  - [x] feat/jun15-mcp-pivot branch 作成 + scaffold commit (= 本 commit)
  - [ ] MCP server endpoint (= `internal/session/mcp_server.go`)
    skeleton + `phonewave mcp` cobra subcommand
  - [ ] phonewave.ping / outbox_status / inbox_status 等の MCP
    tool **interface fixed + stub** (= visibility のみ、 deprecate
    stub なし)
  - [ ] docs(adr): `docs/adr/0006-mcp-pivot.md` 起票 + handover
    finalize
  - [ ] sub-D (post-merge): docs/cli regen if needed

## Next Actions

次 commit で MCP server skeleton 着手:

1. `internal/session/mcp_server.go` を新規実装 (= dominator 59c40b9
   を copy + phonewave 用 adapt)
2. `internal/cmd/mcp.go` cobra subcommand
3. root.go に `newMCPCommand()` register
4. test 配置

## Known Risks / Blockers

- phonewave は LLM 非使用のため、 4 ツール他で必要だった sub-A
  (claude --print deprecate) / sub-B (excludes + test 削除) / sub-D
  (e2e t.Skip) work が全て不要。 commit 数は 4-5 commit と少なめに
  なる。
- D-Mail envelope schema は phonewave が route のみで envelope
  decode しないため、 9-field envelope の domain 実装は **追加しない**
  (= 4 ツールに比べて symmetric 性は劣るが、 phonewave の
  responsibility が body-agnostic な courier daemon である自然な反映)。
- 既存 ADR 0001-0005 は無関係なので、 新 ADR は 0006。

## Context the Next Actor Needs

- **canonical plan**: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- **paintress ADR 0017** / **sightjack ADR 0018** / **amadeus ADR 0026** / **dominator ADR 0003**
- **billing boundary 原則**: LLM 発火は常に human-initiated。 phonewave は LLM 発火しないが、
  visibility tools を MCP として exposes することで 5 ツール pattern
  symmetric を完成させる
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 5 rule、
  preventive (= 既存 違反 なし) なので transitional exclude も初日 から なし
- **MCP server tool 命名規約**: `<tool_name>.<verb>` (= dot 区切り、
  paintress / sightjack / amadeus / dominator と対称)

## Relevant Files and Commands

- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (5
  rule、 preventive only)
- `just lint` - full lint
- `just semgrep` - semgrep gate (= 0 findings 維持目標)
- `just test` - phonewave test suite
