# Cross-Tool Scenario Tests Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** sightjack, paintress, amadeus の各リポジトリに独立した scenario tests (L1-L4) を配置し、各ツールの D-Mail 生成を実行検証する。

**Architecture:** 各ツールの `tests/scenario/` に `//go:build scenario` タグ付き Go テストを配置。phonewave の既存 scenario test パターン（Workspace, Observer, fake-claude, fake-gh）をベースにアダプト。各ツールは自分 + phonewave + fake-claude + fake-gh をビルドし、D-Mail 生成 → phonewave ルーティング → inbox 到着を検証する。

**Tech Stack:** Go, cobra CLI, Netflix/go-expect (sightjack), YAML frontmatter D-Mail, fsnotify, SQLite (modernc.org/sqlite)

**Reference:**
- Design doc: `phonewave/docs/plans/2026-03-02-cross-tool-scenario-tests-design.md`
- Scenario ref: `/Users/nino/tap/refs/scenario-test-plans.md`
- phonewave scenario tests (template): `phonewave/tests/scenario/`

---

## Shared Patterns (from phonewave/tests/scenario/)

以下のファイルは phonewave の実装をベースにコピーアダプトする。各ツールで異なる点のみ Task 内で明記。

| phonewave source | 各ツールへの適用 |
|------------------|----------------|
| `scenario_test.go` | TestMain: ビルド対象を 自分+phonewave+fake-claude+fake-gh に変更 |
| `harness_test.go` | Workspace: 全ツール init は同一。ツール固有の Run/Config ヘルパー追加 |
| `observer_test.go` | そのままコピー（変更不要） |
| `testdata/fake-gh/main.go` | そのままコピー（変更不要） |
| FormatDMail, parseFrontmatter | そのままコピー（変更不要） |

---

## Phase 1: amadeus (最もシンプル — 非対話、Linear 不要)

### Task 1-1: amadeus scenario infrastructure

**Files:**
- Create: `amadeus/tests/scenario/scenario_test.go`
- Create: `amadeus/tests/scenario/harness_test.go`
- Create: `amadeus/tests/scenario/observer_test.go`
- Create: `amadeus/tests/scenario/testdata/fake-claude/main.go`
- Create: `amadeus/tests/scenario/testdata/fake-gh/main.go`
- Create: `amadeus/tests/scenario/testdata/fixtures/minimal/` (fixture files)

**scenario_test.go:**
phonewave 版をベースに、ビルド対象を変更:

```go
//go:build scenario

package scenario_test

// buildAllBinaries builds: amadeus, phonewave, fake-claude (as "claude"), fake-gh (as "gh")
// - amadeus: from this repo (repoPath("AMADEUS_REPO", "amadeus"))
// - phonewave: from sibling (repoPath("PHONEWAVE_REPO", "phonewave"))
// - fake-claude, fake-gh: from testdata/
```

ビルド対象:
- `amadeus` ← `{amadeusRepo}/cmd/amadeus/`
- `phonewave` ← `{phonewaveRepo}/cmd/phonewave/`
- `sightjack` ← `{sightjackRepo}/cmd/sightjack/` (init に必要)
- `paintress` ← `{paintressRepo}/cmd/paintress/` (init に必要)
- `claude` ← `testdata/fake-claude/`
- `gh` ← `testdata/fake-gh/`

Note: Workspace の init で全ツール init が必要（phonewave route 導出のため）。よって全ツールをビルドする。repoPath() は phonewave 版と同じロジック（cwd → 2 階層上 → sibling）。ただし amadeus の tests/scenario/ からの相対パスは:
```
here = amadeus/tests/scenario/
Dir(1) = amadeus/tests/
Dir(2) = amadeus/
Dir(3) = /Users/nino/tap/  ← sibling repos
```

**harness_test.go:**
phonewave 版をそのままコピーし、以下を追加/変更:

1. `RunAmadeus` メソッドは既存のものをそのまま使用
2. amadeus 固有のヘルパー追加:
```go
// RunAmadeusCheck runs amadeus check with --auto-approve and waits for completion.
func (w *Workspace) RunAmadeusCheck(t *testing.T, ctx context.Context, extraArgs ...string) error {
    args := []string{"check", "--auto-approve"}
    args = append(args, extraArgs...)
    args = append(args, w.RepoPath)
    return w.RunAmadeus(t, ctx, args...)
}
```

3. amadeus config override:
```go
// overrideAmadeusClaudeCommand ensures .gate/config.yaml has claude_cmd = "claude"
func (w *Workspace) overrideAmadeusClaudeCommand(t *testing.T) {
    // Read .gate/config.yaml, set claude_cmd to "claude", write back
}
```

**fake-claude (amadeus protocol — stdin→stdout JSON):**
amadeus の `tests/e2e/fake-claude/main.go` をベースにアダプト:

```go
// main.go — amadeus fake-claude (stdin pipe protocol)
// 1. Read prompt from stdin
// 2. Match keywords: "FULL calibration" → fullCalibrationResponse
//    "Changes Since Last Check" → diffCheckResponse
//    default → defaultCleanResponse
// 3. Write JSON to stdout
// Env vars:
//   FAKE_CLAUDE_PROMPT_LOG_DIR — log prompts for inspection
//   FAKE_CLAUDE_FIXTURE_DIR — (optional) read fixtures from files
//   FAKE_CLAUDE_FAIL_PATTERN — simulate failure
//   FAKE_CLAUDE_FAIL_COUNT — fail N times then succeed
```

fullCalibrationResponse は D-Mails を含む JSON (action: "resolve" と severity: "MEDIUM")。amadeus はこれをパースして feedback D-Mail を .gate/outbox/ に書き出す。

**fake-gh:** phonewave 版をそのままコピー。

**Fixtures:** `testdata/fixtures/minimal/` に amadeus の E2E fixture JSON をコピー。

**Commit:**
```
amadeus: tests: scaffold scenario test infrastructure [STRUCTURAL]
```

### Task 1-2: amadeus L1 Minimal

**Files:**
- Create: `amadeus/tests/scenario/minimal_test.go`

**TestScenario_L1_Minimal:**
```go
func TestScenario_L1_Minimal(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    ws := NewWorkspace(t, "minimal")
    obs := NewObserver(ws, t)

    // Start phonewave daemon
    pw := ws.StartPhonewave(t, ctx)
    defer ws.StopPhonewave(t, pw)
    defer ws.DumpPhonewaveLog(t, pw)

    // Inject a report D-Mail into .gate/inbox (upstream input)
    report := FormatDMail(map[string]string{
        "dmail-schema-version": "1",
        "name":                 "report-test-001",
        "kind":                 "report",
        "description":          "Test expedition report",
    }, "# Test Report\n\n## Results\n\n- TEST-001: implemented")
    ws.InjectDMail(t, ".gate", "inbox", "report-test-001.md", report)

    // Run amadeus check — exit code 2 = drift detected (D-Mails generated, normal)
    err := ws.RunAmadeusCheck(t, ctx)
    if err != nil {
        // exit code 2 is expected: amadeus returns 2 when drift is detected
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
            t.Logf("amadeus check returned exit code 2 (drift detected) — expected")
        } else {
            t.Fatalf("amadeus check failed unexpectedly: %v", err)
        }
    }

    // Wait for feedback D-Mail in .gate/outbox → phonewave → .siren/inbox + .expedition/inbox
    ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
    ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)

    // Verify outbox is cleaned up
    ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

    // Verify feedback kind
    feedbackPath := ws.WaitForDMail(t, ".siren", "inbox", 5*time.Second)
    obs.AssertDMailKind(feedbackPath, "feedback")

    obs.AssertAllOutboxEmpty()
}
```

**合格条件:**
- amadeus check が report を消費し、feedback D-Mail を .gate/outbox/ に生成
- phonewave が feedback を .siren/inbox/ と .expedition/inbox/ に fan-out 配送
- 全 outbox が空

**Commit:**
```
amadeus: tests: add L1 minimal scenario test [BEHAVIORAL]
```

### Task 1-3: amadeus L2 Small

**Files:**
- Create: `amadeus/tests/scenario/small_test.go`
- Create: `amadeus/tests/scenario/testdata/fixtures/small/`

**TestScenario_L2_Small:**
- 2 reports inject (異なる priority)
- amadeus check → 2 feedback D-Mails (action: retry + resolve)
- 2 サイクル: retry 後に再 check

**Commit:**
```
amadeus: tests: add L2 small scenario test [BEHAVIORAL]
```

### Task 1-4: amadeus L3 Middle

**Files:**
- Create: `amadeus/tests/scenario/middle_test.go`
- Create: `amadeus/tests/scenario/testdata/fixtures/middle/`

**TestScenario_L3_Middle:**
- 3 reports inject
- amadeus check 2 回連続
- convergence D-Mail を .gate/outbox から inject
- history 蓄積検証

**Commit:**
```
amadeus: tests: add L3 middle scenario test [BEHAVIORAL]
```

### Task 1-5: amadeus L4 Hard

**Files:**
- Create: `amadeus/tests/scenario/hard_test.go`
- Create: `amadeus/tests/scenario/testdata/fixtures/hard/`

**TestScenario_L4_Hard:**
- phonewave daemon 再起動
- FAKE_CLAUDE_FAIL_COUNT=2 で fake-claude 一時失敗
- malformed D-Mail in .gate/inbox
- 回復後の正常動作検証

**Commit:**
```
amadeus: tests: add L4 hard scenario test [BEHAVIORAL]
```

### Task 1-6: amadeus just recipes + verification

**Files:**
- Modify: `amadeus/justfile`

**追加レシピ:** phonewave 版と同一パターン（test-scenario-min, test-scenario-small, test-scenario-middle, test-scenario-hard, test-scenario, test-scenario-all）。

**検証:**
```bash
cd /Users/nino/tap/amadeus
just test-scenario-all
```

**Commit:**
```
amadeus: tests: add just recipes for scenario tests [STRUCTURAL]
```

---

## Phase 2: paintress (自動実行、flag.md 処理が必要)

### Task 2-1: paintress scenario infrastructure

**Files:**
- Create: `paintress/tests/scenario/scenario_test.go`
- Create: `paintress/tests/scenario/harness_test.go`
- Create: `paintress/tests/scenario/observer_test.go`
- Create: `paintress/tests/scenario/testdata/fake-claude/main.go`
- Create: `paintress/tests/scenario/testdata/fake-gh/main.go`
- Create: `paintress/tests/scenario/testdata/fixtures/minimal/`

**ビルド対象:** amadeus 版と同じ構成（全 4 ツール + fake-claude + fake-gh）。

**harness 固有ヘルパー:**
```go
// RunPaintressExpedition runs paintress run with auto-approve, no-dev, workers 0.
func (w *Workspace) RunPaintressExpedition(t *testing.T, ctx context.Context, extraArgs ...string) error {
    args := []string{"run", "--auto-approve", "--no-dev", "--workers", "0"}
    args = append(args, extraArgs...)
    args = append(args, w.RepoPath)
    return w.RunPaintress(t, ctx, args...)
}

// overridePaintressClaudeCommand ensures .expedition/config.yaml has claude_cmd = "claude"
func (w *Workspace) overridePaintressClaudeCommand(t *testing.T) { ... }
```

**fake-claude (paintress protocol — stdout text):**
paintress の `tests/e2e/fake-claude/main.go` をベースにアダプト:

```go
// main.go — paintress fake-claude (stdout text protocol)
// 1. Parse -p flag for prompt
// 2. Output canned expedition report text to stdout
// 3. Env vars: FAKE_CLAUDE_PROMPT_LOG_DIR, FAKE_CLAUDE_FAIL_PATTERN
// Key behavior:
//   - Read prompt to find spec/issue info
//   - Update flag.md with issue ID (simulates Claude picking an issue)
//   - Output valid expedition report text
//   - Create a simple git commit (simulates code changes)
```

paintress の fake-claude は stdout にテキストを返すだけでなく、以下のシミュレーションも行う:
1. `flag.md` に issue ID を書き込み（paintress が expedition 進捗を追跡するため）
2. 簡単な git commit を作成（paintress が diff を検出するため）
3. expedition report テキストを stdout に出力

**Commit:**
```
paintress: tests: scaffold scenario test infrastructure [STRUCTURAL]
```

### Task 2-2: paintress L1 Minimal

**Files:**
- Create: `paintress/tests/scenario/minimal_test.go`

**TestScenario_L1_Minimal:**
```go
func TestScenario_L1_Minimal(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    ws := NewWorkspace(t, "minimal")
    obs := NewObserver(ws, t)
    pw := ws.StartPhonewave(t, ctx)
    defer ws.StopPhonewave(t, pw)
    defer ws.DumpPhonewaveLog(t, pw)

    // Inject specification D-Mail into .expedition/inbox (upstream input)
    spec := FormatDMail(map[string]string{
        "dmail-schema-version": "1",
        "name":                 "spec-test-001",
        "kind":                 "specification",
        "description":          "Test specification",
    }, "# Test Spec\n\n## Actions\n\n- [add_dod] TEST-001: Test action")
    ws.InjectDMail(t, ".expedition", "inbox", "spec-test-001.md", spec)

    // Run paintress expedition
    ws.RunPaintressExpedition(t, ctx)

    // Wait for report D-Mail delivery
    ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)
    ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)

    // Verify report kind
    reportPath := ws.WaitForDMail(t, ".gate", "inbox", 5*time.Second)
    obs.AssertDMailKind(reportPath, "report")

    obs.AssertAllOutboxEmpty()
}
```

**合格条件:**
- paintress が specification を消費し、expedition 実行後に report D-Mail を .expedition/outbox/ に生成
- phonewave が report を .gate/inbox/ に配送
- 全 outbox が空

**Commit:**
```
paintress: tests: add L1 minimal scenario test [BEHAVIORAL]
```

### Task 2-3: paintress L2 Small

**TestScenario_L2_Small:**
- 2 specs inject + feedback (retry) inject
- paintress expedition → 2 reports
- follow-up expedition 検証

**Commit:**
```
paintress: tests: add L2 small scenario test [BEHAVIORAL]
```

### Task 2-4: paintress L3 Middle

**TestScenario_L3_Middle:**
- 3 specs, `--workers 2` 並列
- interleaved D-Mails
- convergence 検証

**Commit:**
```
paintress: tests: add L3 middle scenario test [BEHAVIORAL]
```

### Task 2-5: paintress L4 Hard

**TestScenario_L4_Hard:**
- fake-claude 失敗 → escalation D-Mail 検証
- phonewave daemon 再起動
- malformed inbox D-Mail

**Commit:**
```
paintress: tests: add L4 hard scenario test [BEHAVIORAL]
```

### Task 2-6: paintress just recipes + verification

**Commit:**
```
paintress: tests: add just recipes for scenario tests [STRUCTURAL]
```

---

## Phase 3: sightjack (go-expect 対話、最も複雑)

### Task 3-1: sightjack scenario infrastructure

**Files:**
- Create: `sightjack/tests/scenario/scenario_test.go`
- Create: `sightjack/tests/scenario/harness_test.go`
- Create: `sightjack/tests/scenario/observer_test.go`
- Create: `sightjack/tests/scenario/testdata/fake-claude/main.go`
- Create: `sightjack/tests/scenario/testdata/fake-gh/main.go`
- Create: `sightjack/tests/scenario/testdata/fixtures/minimal/`

**ビルド対象:** 全 4 ツール + fake-claude + fake-gh。

**harness 固有ヘルパー (go-expect):**

sightjack は interactive stdin を要求するため、go-expect で PTY をシミュレートする。sightjack の `tests/e2e/interactive_test.go` パターンを参照。

```go
import "github.com/Netflix/go-expect"

// RunSightjackInteractive runs sightjack run with go-expect PTY.
// interactions is a sequence of (expect, send) pairs.
func (w *Workspace) RunSightjackInteractive(t *testing.T, ctx context.Context,
    interactions []Interaction, extraArgs ...string) error {

    c, err := expect.NewConsole(expect.WithDefaultTimeout(30 * time.Second))
    if err != nil {
        t.Fatalf("create console: %v", err)
    }
    defer c.Close()

    args := []string{"run"}
    args = append(args, extraArgs...)
    args = append(args, w.RepoPath)
    cmd := w.runToolCmd(ctx, "sightjack", args...)
    // CRITICAL: Set SIGHTJACK_TTY to the go-expect PTY device.
    // Without this, sightjack opens /dev/tty directly and ignores cmd.Stdin.
    cmd.Env = append(cmd.Env, "SIGHTJACK_TTY="+c.Tty().Name())
    cmd.Stdin = c.Tty()
    cmd.Stdout = c.Tty()
    cmd.Stderr = c.Tty()

    if err := cmd.Start(); err != nil {
        t.Fatalf("start sightjack: %v", err)
    }

    for _, i := range interactions {
        if _, err := c.ExpectString(i.Expect); err != nil {
            t.Fatalf("expect %q: %v", i.Expect, err)
        }
        if err := c.SendLine(i.Send); err != nil {
            t.Fatalf("send %q: %v", i.Send, err)
        }
    }

    return cmd.Wait()
}

// Interaction represents an expect/send pair for go-expect.
type Interaction struct {
    Expect string // string to wait for
    Send   string // string to send (with newline)
}
```

Note: sightjack の go.mod に既に `github.com/Netflix/go-expect` がある。scenario テストも同じ module 内なので追加不要。

**fake-claude (sightjack protocol — file-based JSON):**
sightjack の `tests/e2e/fake-claude/main.go` をベースにアダプト:

```go
// main.go — sightjack fake-claude (file-based JSON protocol)
// 1. Parse -p flag for prompt
// 2. Extract JSON file path from prompt (regex: /[^\s"]+\.json)
// 3. Match filename pattern → fixture
//    classify.json → classifySingleCluster
//    cluster_*_c*.json → deepScanAuth
//    wave_*_*.json → waveGenAuth
//    apply_*_*.json → waveApplySuccess
//    nextgen_*_*.json → nextgenEmpty
// 4. Write fixture JSON to extracted path
// Env vars: FAKE_CLAUDE_PROMPT_LOG_DIR, FAKE_CLAUDE_FAIL_PATTERN, FAKE_CLAUDE_FAIL_COUNT
```

**Commit:**
```
sightjack: tests: scaffold scenario test infrastructure [STRUCTURAL]
```

### Task 3-2: sightjack L1 Minimal

**Files:**
- Create: `sightjack/tests/scenario/minimal_test.go`

**TestScenario_L1_Minimal:**
```go
func TestScenario_L1_Minimal(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    ws := NewWorkspace(t, "minimal")
    obs := NewObserver(ws, t)
    pw := ws.StartPhonewave(t, ctx)
    defer ws.StopPhonewave(t, pw)
    defer ws.DumpPhonewaveLog(t, pw)

    // Run sightjack with go-expect for interactive wave selection
    interactions := []Interaction{
        {Expect: "Select wave", Send: "1"},   // Select first wave
        {Expect: "Approve", Send: "a"},         // Approve all
    }
    err := ws.RunSightjackInteractive(t, ctx, interactions,
        "--approve-cmd", "exit 0")
    if err != nil {
        t.Fatalf("sightjack run failed: %v", err)
    }

    // Wait for specification D-Mail delivery
    specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
    ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

    // Verify specification kind
    obs.AssertDMailKind(specPath, "specification")

    obs.AssertAllOutboxEmpty()
}
```

**合格条件:**
- sightjack が scan → classify → deep scan → wave gen → approve → spec D-Mail 生成
- phonewave が spec を .expedition/inbox/ に配送
- 全 outbox が空

**Commit:**
```
sightjack: tests: add L1 minimal scenario test [BEHAVIORAL]
```

### Task 3-3: sightjack L2 Small

**TestScenario_L2_Small:**
- 2 clusters (fake-claude fixture で 2 cluster 返却)
- selective approve (go-expect で "s" → issue 選択)
- priority 検証

**Commit:**
```
sightjack: tests: add L2 small scenario test [BEHAVIORAL]
```

### Task 3-4: sightjack L3 Middle

**TestScenario_L3_Middle:**
- convergence D-Mail を .siren/inbox/ に inject
- convergence gate 通過後 scan
- 3+ issues

**Commit:**
```
sightjack: tests: add L3 middle scenario test [BEHAVIORAL]
```

### Task 3-5: sightjack L4 Hard

**TestScenario_L4_Hard:**
- FAKE_CLAUDE_FAIL_PATTERN で一時失敗
- malformed inbox
- daemon 再起動後のリカバリ

**Commit:**
```
sightjack: tests: add L4 hard scenario test [BEHAVIORAL]
```

### Task 3-6: sightjack just recipes + verification

**Commit:**
```
sightjack: tests: add just recipes for scenario tests [STRUCTURAL]
```

---

## Task Dependencies

```
Phase 1 (amadeus)        Phase 2 (paintress)     Phase 3 (sightjack)
  1-1 infra                2-1 infra               3-1 infra
    |                        |                        |
  1-2 L1                   2-2 L1                  3-2 L1
    |                        |                        |
  1-3 L2                   2-3 L2                  3-3 L2
    |                        |                        |
  1-4 L3                   2-4 L3                  3-4 L3
    |                        |                        |
  1-5 L4                   2-5 L4                  3-5 L4
    |                        |                        |
  1-6 recipes              2-6 recipes             3-6 recipes
```

**3 Phase は実行順序として独立** — どの Phase から着手してもよい。ただし全 Phase がビルド時に 4 sibling repos を必要とする（phonewave route 導出に全ツールの SKILL.md が必要なため）。Phase 内は sequential。

## Execution Order (推奨)

1. **amadeus first** — 最もシンプル。パターンを確立。
2. **paintress second** — flag.md + git commit シミュレーションの複雑さを吸収。
3. **sightjack last** — go-expect 対話が最も複雑。

## Key Implementation Notes

1. **全ツール init が必要** — phonewave の route 導出には全ツールの SKILL.md が必要。各 scenario test の TestMain で全 4 ツールをビルドする。

2. **repoPath() のパス計算** — 各ツールの `tests/scenario/` から `Dir(3)` で `/Users/nino/tap/` に到達。phonewave と同じロジック。

3. **fake-claude は各ツール専用** — phonewave の unified fake-claude とは異なり、各ツールは自分のプロトコルだけを処理する fake-claude を持つ。

4. **amadeus の exit code** — exit 0 = success (no drift), exit 2 = drift detected (D-Mails generated)。exit 2 は正常動作であり、テスト内では `*exec.ExitError` の `ExitCode() == 2` をチェックして正常として扱う。

5. **paintress の fake-claude** — expedition report だけでなく flag.md 更新と git commit も必要。最も複雑な fake-claude。

6. **sightjack の go-expect** — sightjack の go.mod に既に go-expect がある。scenario テストは同じ module 内。

7. **Config override** — 各ツールの config で claude command を "claude" に上書き。ツールごとに config 構造が異なるので個別対応。
