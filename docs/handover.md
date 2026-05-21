# Handover

**Last updated:** 2026-05-21 (asia/tokyo, Phase 2d finalize)
**Updated by:** Claude Opus 4.7 session

## Current State

`feat/jun15-mcp-pivot` long-lived branch 上で refs/issues/0027
(jun15 MCP pivot v4) の Phase 2d (= phonewave optional visibility
MCP) を 4 commit (scaffold + skeleton + docs/cli + ADR/handover)
完了。 5 ツール pattern (paintress / sightjack / amadeus / dominator
/ phonewave) symmetric を確立。

Phase 2d 完了内容:

1. **`.semgrep/jun15-no-headless-llm.yaml`** (= 5 rule, preventive
   gate のみ、 transitional exclude なし — phonewave は元から
   `claude --print` 不使用)。
2. **`phonewave mcp` MCP server** (`internal/session/mcp_server.go`)
   = JSON-RPC 2.0 stdio、 4 MiB scanner buffer、 Phase 2d MVP として
   `phonewave.ping` / `phonewave.outbox_status` /
   `phonewave.inbox_status` を advertise + dispatch。 後 2 つは
   visibility contract 固定 + stub。 5 test pass。
3. **`phonewave mcp` cobra subcommand** (`internal/cmd/mcp.go`)
   = 5 ツール全 attach Example 提示 + LLM 非使用 / 純粋 additive
   旨を明示。
4. **docs/cli regen** (= sub-D-1 相当、 docs/cli/phonewave_mcp.md
   新規 + 14 既存 file の tab 形式 restore)。
5. **ADR 0006-mcp-pivot.md** (= 5 ツール pattern 完成のための
   architectural pin、 4 ツール ADR (paintress 0017 / sightjack
   0018 / amadeus 0026 / dominator 0003) の symmetric counterpart)。

## In Progress

- branch: `feat/jun15-mcp-pivot` (= scaffold + 3 commit + ADR + 本
  handover = 5 commit、 sub-D は必要時 post-merge fixup)
- main merge は Phase 2d 完了後の PR 作成 + CI green +
  squash-merge 待ち
- 次 phase: refs/issues/0027 の Phase 2 (= 5 ツール横展開) 全完了
  → Phase 3 finalize (= 公式 docs / migration guide / lessons
  learned 集約) + MCP tool stubs の real implementation 化 +
  cost monitoring 3 軸検証

## Next Actions

1. `feat/jun15-mcp-pivot` に PR 作成 (= title: `feat(session):
   Phase 2d phonewave optional MCP visibility (refs/issues/0027)`)
2. CI を green まで監視 (= LLM 非使用なので docs-check 以外の
   risk は小さい)
3. squash-merge 完了後、 refs/issues/0027 を 5 ツール全完了に更新
4. 次 phase へ (= real implementation + cost monitoring + Phase 3)

## Known Risks / Blockers

- phonewave は LLM 使用なし + envelope decode なし + skill なしで
  4 ツールに比べて軽量。 deprecate stub / 既存 test 削除 / e2e
  t.Skip 等の cleanup work が全くないため CI fail の risk が低い。
- ただし markdownlint と docgen が tab/space で互いに反発する quirk
  に注意 (= docs/cli/ は docgen 出力 = tab 形式を canonical とする)。

## Context the Next Actor Needs

- **canonical plan**: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- **paintress ADR 0017** / **sightjack ADR 0018** / **amadeus ADR 0026** / **dominator ADR 0003**
- **phonewave ADR 0006**: `docs/adr/0006-mcp-pivot.md` (= 5 ツール
  pattern 完成、 visibility-only)
- **billing boundary 原則**: phonewave は LLM 非使用なので boundary
  には触れないが、 5 ツール symmetric のために MCP server を追加
  (= refs 0027 §4 'optional: visibility' 指定)
- **semgrep gate**: `.semgrep/jun15-no-headless-llm.yaml` 5 rule、
  preventive only (= 既存 違反 なし)
- **MCP server tool 命名規約**: `<tool_name>.<verb>` (= 5 ツール
  全 dot 区切り対称)

## Relevant Files and Commands

- `docs/adr/0006-mcp-pivot.md` - 本 phase の architectural pin
- `.semgrep/jun15-no-headless-llm.yaml` - billing-boundary gate (5
  rule、 preventive only)
- `internal/session/mcp_server.go` - phonewave MCP server (= Phase 2d
  MVP scope、 3 tool stub)
- `internal/cmd/mcp.go` - `phonewave mcp` cobra subcommand
- `docs/cli/phonewave_mcp.md` - subcommand reference (= docgen 出力)
- `just lint` - full lint
- `just test` - phonewave test suite (= 全 pkg ok)
- `just semgrep` - semgrep gate (= 0 findings 維持、 74 rules)
