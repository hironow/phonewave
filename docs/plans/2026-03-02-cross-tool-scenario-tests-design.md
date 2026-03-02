# Cross-Tool Scenario Tests Design

## Goal

sightjack, paintress, amadeus の各ツールに独立した scenario tests (L1-L4) を配置し、各ツールの D-Mail 生成を実行検証する。phonewave の既存 scenario tests（inject-based routing 検証）を補完し、closed loop の全区間をカバーする。

## Architecture

各ツールの `tests/scenario/` に `//go:build scenario` タグ付きの Go テストを配置。自分のツールを実際に実行し、D-Mail 生成の正しさと phonewave によるルーティングを検証する。

### Approach: 各ツール独立 scenario tests

```
sightjack/tests/scenario/   -- sightjack実行 → spec D-Mail生成検証
paintress/tests/scenario/    -- paintress実行 → report D-Mail生成検証
amadeus/tests/scenario/      -- amadeus実行 → feedback D-Mail生成検証
phonewave/tests/scenario/    -- (既存) inject-based routing検証
```

### Fake Externals

- `fake-claude`: 各ツールの E2E 版をベースにアダプト（tool-specific protocol）
- `fake-gh`: phonewave 版パターンをコピー
- Linear API: 不要（`paintress run` は Linear を直接呼ばない）

### Test Boundaries

| ツール | 実行検証 | 上流入力 | 下流検証 |
|--------|---------|---------|---------|
| sightjack | scan → spec D-Mail生成 | (自身がスキャン) | phonewave → .expedition/inbox |
| paintress | spec消費 → report D-Mail生成 | spec inject | phonewave → .gate/inbox |
| amadeus | report消費 → feedback D-Mail生成 | report inject | phonewave → .siren/inbox + .expedition/inbox |

## Directory Structure (per tool)

```
{tool}/tests/scenario/
  scenario_test.go          -- TestMain: build binaries
  harness_test.go           -- Workspace, tool lifecycle helpers
  observer_test.go          -- assertions
  minimal_test.go           -- L1
  small_test.go             -- L2
  middle_test.go            -- L3
  hard_test.go              -- L4
  testdata/
    fake-claude/main.go     -- tool-specific fake-claude
    fake-gh/main.go         -- shared pattern
    fixtures/{minimal,small,middle,hard}/
```

## Binary Builds (TestMain)

| テスト場所 | ビルド対象 |
|-----------|-----------|
| sightjack/tests/scenario | sightjack, phonewave, fake-claude, fake-gh |
| paintress/tests/scenario | paintress, phonewave, fake-claude, fake-gh |
| amadeus/tests/scenario | amadeus, phonewave, fake-claude, fake-gh |

## Workspace Setup (shared pattern)

1. `t.TempDir()` → `git init` → initial commit
2. 全ツール init: `sightjack init`, `paintress init`, `amadeus init`, `phonewave init`
3. phonewave は SKILL.md をスキャンして routes 自動導出
4. 各ツール config の claude command を `claude` (fake) に上書き
5. Route 導出検証: specification, report, feedback の 3 route が存在することを assert

## Tool-Specific Details

### sightjack

- **Interactive stdin**: wave選択に go-expect (Netflix/go-expect) で PTY シミュレーション
- **fake-claude protocol**: file-based JSON（prompt 内の JSON path を抽出→ファイル書き込み）
- **既存資産ベース**: `sightjack/tests/e2e/fake-claude/main.go`
- **環境変数**: `SIGHTJACK_TTY` で PTY 差し替え可能

### paintress

- **完全自動実行**: `paintress run --auto-approve --no-dev --workers 0`
- **fake-claude protocol**: stdout text（keyword matching → expedition report 出力）
- **既存資産ベース**: `paintress/tests/e2e/fake-claude/main.go`
- **Linear API**: 不要。Claude Code（fake）が Linear を使う設計だが、fake-claude はそこをスキップ
- **flag.md**: fake-claude が expedition 中に更新する必要あり

### amadeus

- **完全自動実行**: `amadeus check --auto-approve`
- **fake-claude protocol**: stdin→stdout JSON（pipe 経由でプロンプト受信、calibration JSON 返却）
- **既存資産ベース**: `amadeus/tests/e2e/fake-claude/main.go`
- **最も素直**: 対話不要、Linear 不要

## L1-L4 Scenarios

### sightjack

| Level | 内容 | go-expect 対話 |
|-------|------|---------------|
| L1 | 1 issue, 1 wave, scan→approve→spec D-Mail | "1"→"a" |
| L2 | 2 issues, priority差、selective approve | wave選択→selective |
| L3 | 3+ issues, convergence inbox消費, gate通過後scan | gate auto + wave選択 |
| L4 | fake-claude失敗, malformed inbox, daemon再起動 | リカバリ検証 |

### paintress

| Level | 内容 |
|-------|------|
| L1 | 1 spec inject→expedition→report D-Mail→phonewave→.gate/inbox |
| L2 | 2 specs + feedback(retry) inject→follow-up expedition |
| L3 | 3+ specs, --workers 2 並列, interleaved D-Mails |
| L4 | fake-claude失敗→escalation D-Mail, daemon再起動 |

### amadeus

| Level | 内容 |
|-------|------|
| L1 | 1 report inject→check→feedback D-Mail→phonewave→fan-out (2箇所) |
| L2 | 2 reports, mixed actions (retry/resolve), severity |
| L3 | multiple checks連続, convergence, history蓄積 |
| L4 | fake-claude失敗, high severity gate, daemon再起動 |

## Just Recipes (per tool)

```just
test-scenario-min:
    go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s

test-scenario-small:
    go test -tags scenario ./tests/scenario/ -run TestScenario_L2 -count=1 -v -timeout=180s

test-scenario-middle:
    go test -tags scenario ./tests/scenario/ -run TestScenario_L3 -count=1 -v -timeout=300s

test-scenario-hard:
    go test -tags scenario ./tests/scenario/ -run TestScenario_L4 -count=1 -v -timeout=600s

test-scenario:
    go test -tags scenario ./tests/scenario/ -run "TestScenario_L[12]" -count=1 -v -timeout=300s

test-scenario-all:
    go test -tags scenario ./tests/scenario/ -count=1 -v -timeout=900s
```

## Design Decisions

1. **各ツール独立** — CI で変更検知し自分のシナリオテストを実行。phonewave 側のテストは routing のみ担当。
2. **fake externals only** — claude, gh, linear のみ fake。ツール本体、phonewave daemon、D-Mail 処理は全て実物。
3. **上流 D-Mail は inject** — 各ツールは自分の D-Mail 生成のみを検証責務とする。上流ツールの出力テストはそのツール側の責務。
4. **harness はコピーアダプト** — 共有モジュール作成は over-engineering。test-only コードの DRY より独立性を優先。
5. **phonewave の既存テストは残す** — inject-based routing 検証は phonewave の責務として引き続き有効。
