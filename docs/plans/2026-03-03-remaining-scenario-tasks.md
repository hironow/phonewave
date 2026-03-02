# Remaining Scenario Test Tasks Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** `/Users/nino/tap/refs/remain-todo-scenario-test.md` の全残項目を実装し、4ツール closed loop を完全検証する。

**Architecture:** 各ツールの scenario test package に既存の harness (4 tool build + phonewave daemon) を活用し、L1 拡張 + WaitForClosedLoop 導入 + approve-cmd テスト + convergence 契約化 + L2-L4 実経路化を実施。

**Tech Stack:** Go, cobra CLI, scenario build tag, fake-claude/fake-gh fixtures

---

## Context

前回セッションで 3 gap fixes (SKILL.md feedback route, fixture markers, sightjack auto-approve) を完了。
sightjack/paintress/amadeus の L1-L4 全 PASS を確認済み。残りは refs/ の TODO 項目。

## Current L1 Test State

- **phonewave L1**: outbox 直接注入 → routing 確認 (tool 未実行)
- **sightjack L1**: sightjack scan → specification 生成 → phonewave delivery で終了
- **paintress L1**: spec inject → paintress run → report 生成 → phonewave delivery で終了
- **amadeus L1**: report inject → amadeus check → feedback 生成 → phonewave delivery で終了

## Convergence Route Contract

SKILL.md からの route 導出:
- convergence: amadeus (.gate) produces → sightjack (.siren) consumes
- Expected route: `.gate/outbox` → `.siren/inbox`
- paintress does NOT consume convergence → `.expedition/inbox` に配送されないことを assert

---

## Task 0: Add missing cross-tool harness helpers [STRUCTURAL]

**Files:**
- Modify: `sightjack/tests/scenario/harness_test.go` — add `RunPaintressExpedition` (NOTE: `RunAmadeusCheck` already exists)
- Modify: `paintress/tests/scenario/harness_test.go` — add `RunSightjackScan`
- Modify: `amadeus/tests/scenario/harness_test.go` — add `RunSightjackScan`, `RunPaintressExpedition`
- Modify: `phonewave/tests/scenario/harness_test.go` — add `RunSightjackScan`, `RunPaintressExpedition`, `RunAmadeusCheck`

**Current helper inventory:**
- sightjack: `RunSightjack` (generic), `RunSightjackScan` (convenience), `RunPaintress`, `RunAmadeus`, `RunAmadeusCheck`
- paintress: `RunSightjack`, `RunPaintress`, `RunPaintressExpedition`, `RunAmadeus`, `RunAmadeusCheck`
- amadeus: `RunSightjack`, `RunPaintress`, `RunAmadeus`, `RunAmadeusCheck`
- phonewave: `RunSightjack`, (no convenience helpers)

**What:**
各 repo の harness に cross-tool convenience helper を追加。パターンは既存の paintress harness に倣う:

```go
// RunSightjackScan runs sightjack run with --auto-approve.
func (w *Workspace) RunSightjackScan(t *testing.T, ctx context.Context, extraArgs ...string) error {
    t.Helper()
    args := []string{"run", "--auto-approve"}
    args = append(args, extraArgs...)
    args = append(args, w.RepoPath)
    return w.RunSightjack(t, ctx, args...)
}

// RunPaintressExpedition runs paintress run with auto-approve, no-dev, workers 0, max-expeditions 1.
func (w *Workspace) RunPaintressExpedition(t *testing.T, ctx context.Context, extraArgs ...string) error {
    t.Helper()
    args := []string{"run", "--auto-approve", "--no-dev", "--workers", "0", "--max-expeditions", "1"}
    args = append(args, extraArgs...)
    args = append(args, w.RepoPath)
    return w.RunPaintress(t, ctx, args...)
}

// RunAmadeusCheck runs amadeus check with --auto-approve.
func (w *Workspace) RunAmadeusCheck(t *testing.T, ctx context.Context, extraArgs ...string) error {
    t.Helper()
    args := []string{"check", "--auto-approve"}
    args = append(args, extraArgs...)
    args = append(args, w.RepoPath)
    return w.RunAmadeus(t, ctx, args...)
}
```

Only add helpers that are MISSING from each repo. Do NOT duplicate existing ones.

**Verification:**
All existing scenario tests must still pass:
```bash
cd /Users/nino/tap/sightjack && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
cd /Users/nino/tap/paintress && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
cd /Users/nino/tap/amadeus && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
cd /Users/nino/tap/phonewave && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
```

**Commit (per repo):**
```
{tool}: tests: add cross-tool convenience helpers to scenario harness [STRUCTURAL]
```

---

## Task 1: T1-2 — WaitForClosedLoop() activation in phonewave L1 [BEHAVIORAL]

**Files:**
- Modify: `phonewave/tests/scenario/minimal_test.go` (line 99-101)

**What:**
phonewave L1 にのみ `obs.WaitForClosedLoop(60 * time.Second)` を追加。
phonewave L1 は既に全 3 route (spec→expedition, report→gate, feedback→siren) を直接注入で検証しているため、WaitForClosedLoop の全条件を満たす。

sightjack/paintress/amadeus の L1 への WaitForClosedLoop 追加は Task 3-5 で full loop 化と同時に行う。
(理由: 現状の各 L1 は closed loop 未完成のため WaitForClosedLoop を先行投入すると timeout で失敗確定)

**Implementation:**
phonewave L1: `obs.AssertAllOutboxEmpty()` の直前に追加:

**Verification:**
```bash
cd /Users/nino/tap/phonewave && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
```

**Commit:**
```
phonewave: tests: add WaitForClosedLoop to L1 scenario test [BEHAVIORAL]
```

---

## Task 2: PW-2 — Convergence route contract [BEHAVIORAL]

**Files:**
- Modify: `phonewave/tests/scenario/middle_test.go` (line 58-74)

**What:**
Phase 2 の convergence D-Mail を `.gate/outbox` から inject し、`.siren/inbox` への配送を明示 assert する。
現在の「Either route or error queue — we just verify the system doesn't deadlock」を契約化。

SKILL.md route 導出:
- amadeus (.gate) produces convergence → sightjack (.siren) consumes convergence
- Route: `.gate/outbox` → `.siren/inbox`

**Implementation:**
```go
// Phase 2: Convergence D-Mail — CONTRACTED to route to .siren/inbox
convergenceDMail := FormatDMail(map[string]string{
    "dmail-schema-version": "1",
    "name":                 "convergence-auth-001",
    "kind":                 "convergence",
    "description":          "Recurring drift in auth module",
    "severity":             "medium",
}, "# Convergence: Auth Module\n\nRecurring issues detected in authentication module across 3 cycles.")

ws.InjectDMail(t, ".gate", "outbox", "convergence-auth-001.md", convergenceDMail)

// Convergence route: .gate/outbox → .siren/inbox (amadeus produces → sightjack consumes)
convPath := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
obs.AssertDMailKind(convPath, "convergence")

// Convergence must NOT fan-out to .expedition/inbox (paintress does not consume convergence)
// .expedition/inbox already has 3 specs from Phase 1
```

Replace `time.Sleep(3 * time.Second)` with actual delivery assertion.

Also update Phase 4 feedback fan-out count: `.siren/inbox` should now be 3 feedbacks + 1 convergence = 4 total.
```go
// Phase 4: Wait for fan-out: 3 feedbacks + 1 convergence already in .siren/inbox = 4
ws.WaitForDMailCount(t, ".siren", "inbox", 4, 30*time.Second)
```

**Verification:**
```bash
cd /Users/nino/tap/phonewave && go test -tags scenario ./tests/scenario/ -run TestScenario_L3 -count=1 -v -timeout=300s
```

**Commit:**
```
phonewave: tests: contract convergence route in L3 scenario test [BEHAVIORAL]
```

---

## Task 3: SJ-1 — Extend sightjack L1 to full closed loop [BEHAVIORAL]

**Files:**
- Modify: `sightjack/tests/scenario/minimal_test.go`

**What:**
sightjack L1 を specification 生成で終わらず、full loop (spec → paintress → report → amadeus → feedback → sightjack inbox) まで確認。

**Implementation:**
```go
func TestScenario_L1_Minimal(t *testing.T) {
    // ... existing setup ...

    // 1. Run sightjack → specification in .siren/outbox → phonewave → .expedition/inbox
    err := ws.RunSightjackScan(t, ctx)
    if err != nil {
        t.Fatalf("sightjack scan failed: %v", err)
    }
    specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
    ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)
    obs.AssertDMailKind(specPath, "specification")

    // 2. Run paintress → report in .expedition/outbox → phonewave → .gate/inbox
    err = ws.RunPaintressExpedition(t, ctx)
    if err != nil {
        t.Fatalf("paintress expedition failed: %v", err)
    }
    reportPath := ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)
    ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)
    obs.AssertDMailKind(reportPath, "report")

    // 3. Run amadeus → feedback in .gate/outbox → phonewave → .siren/inbox + .expedition/inbox
    err = ws.RunAmadeusCheck(t, ctx)
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
            t.Logf("amadeus check returned exit code 2 (drift detected) — expected")
        } else {
            t.Fatalf("amadeus check failed: %v", err)
        }
    }
    feedbackPath := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
    obs.AssertDMailKind(feedbackPath, "feedback")

    // 4. Full closed loop verified
    obs.WaitForClosedLoop(60 * time.Second)
    obs.AssertAllOutboxEmpty()
}
```

**Note:** sightjack harness must have `RunPaintressExpedition` and `RunAmadeusCheck` helpers. Check if they exist — they should from the scenario_test.go TestMain that builds all 4 tools.

**Verification:**
```bash
cd /Users/nino/tap/sightjack && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=180s
```

**Commit:**
```
sightjack: tests: extend L1 to full closed loop (spec→report→feedback) [BEHAVIORAL]
```

---

## Task 4: PT-1 — Extend paintress L1 to full closed loop [BEHAVIORAL]

**Files:**
- Modify: `paintress/tests/scenario/minimal_test.go`

**What:**
paintress L1 を report 生成で終わらず、amadeus feedback 受領後の feedback inbox 到達まで確認。

**Implementation:**
```go
func TestScenario_L1_Minimal(t *testing.T) {
    // ... existing setup (inject spec, run paintress, verify report) ...

    // After report delivery to .gate/inbox:

    // 3. Run amadeus → feedback in .gate/outbox → phonewave → .expedition/inbox + .siren/inbox
    err = ws.RunAmadeusCheck(t, ctx)
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
            t.Logf("amadeus exit code 2 (drift) — expected")
        } else {
            t.Fatalf("amadeus check failed: %v", err)
        }
    }

    // 4. Verify feedback arrived in .expedition/inbox (paintress consumes feedback)
    // .expedition/inbox now has: spec-test-001 + feedback
    ws.WaitForDMailCount(t, ".expedition", "inbox", 2, 30*time.Second)

    // 5. Full loop: spec → report → feedback all delivered
    obs.WaitForClosedLoop(60 * time.Second)
    obs.AssertAllOutboxEmpty()
}
```

**Verification:**
```bash
cd /Users/nino/tap/paintress && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=180s
```

**Commit:**
```
paintress: tests: extend L1 to full closed loop (report→feedback delivery) [BEHAVIORAL]
```

---

## Task 5: AM-1 — Extend amadeus L1 to verify downstream [BEHAVIORAL]

**Files:**
- Modify: `amadeus/tests/scenario/minimal_test.go`

**What:**
amadeus L1 の feedback が `.siren/inbox` と `.expedition/inbox` の両方に配送されることを明示確認。
(現状は `.siren/inbox` と `.expedition/inbox` count=1 を WaitForDMail/WaitForDMailCount で確認済みだが、WaitForClosedLoop は未使用)

**Implementation:**
既存のテストの末尾に WaitForClosedLoop を追加:
```go
    // ... existing feedback verification ...

    // Verify full closed loop delivery
    obs.WaitForClosedLoop(60 * time.Second)
    obs.AssertAllOutboxEmpty()
```

ただし WaitForClosedLoop は .expedition/inbox + .gate/inbox + .siren/inbox の 3 点を確認する。
amadeus L1 は report を .gate/inbox に inject しているので .gate/inbox は OK。
feedback が .siren/inbox と .expedition/inbox に到達するので .siren/inbox も OK。
.expedition/inbox は feedback 到達で OK。
→ 全条件を満たす。

**Verification:**
```bash
cd /Users/nino/tap/amadeus && go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
```

**Commit:**
```
amadeus: tests: add WaitForClosedLoop to L1 for downstream verification [BEHAVIORAL]
```

---

## Task 6: T2-1 — approve-cmd/notify-cmd scenario tests (all 3 tools) [BEHAVIORAL]

**Files:**
- Create: `sightjack/tests/scenario/approve_test.go`
- Create: `paintress/tests/scenario/approve_test.go`
- Create: `amadeus/tests/scenario/approve_test.go`

**What:**
各ツールの scenario test に `--approve-cmd` と `--notify-cmd` を使うテストケースを追加。
`--auto-approve` とは別経路で承認が通ることを検証。

**Implementation pattern (all 3 tools):**
```go
func TestScenario_ApproveCmdPath(t *testing.T) {
    if testing.Short() {
        t.Skip("scenario tests are not short")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    ws := NewWorkspace(t, "minimal")

    pw := ws.StartPhonewave(t, ctx)
    defer ws.StopPhonewave(t, pw)
    defer ws.DumpPhonewaveLog(t, pw)

    // Create approve script (exit 0 = approve)
    approveScript := filepath.Join(ws.Root, "approve.sh")
    os.WriteFile(approveScript, []byte("#!/bin/sh\nexit 0\n"), 0o755)

    // Create notify script (log to file for verification)
    notifyLog := filepath.Join(ws.Root, "notify.log")
    notifyScript := filepath.Join(ws.Root, "notify.sh")
    os.WriteFile(notifyScript, []byte(fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %s\n", notifyLog)), 0o755)

    // Run tool with --approve-cmd and --notify-cmd instead of --auto-approve
    // ... tool-specific run with approval flags ...

    // Verify approval script was used (tool succeeded)
    // Verify notify script was invoked (notify.log exists and non-empty)
}
```

**Per-tool specifics (use GENERIC helpers to avoid --auto-approve hardcoding):**
- sightjack: `ws.RunSightjack(t, ctx, "run", "--approve-cmd", approveScript, "--notify-cmd", notifyScript, ws.RepoPath)`
- paintress: `ws.RunPaintress(t, ctx, "run", "--approve-cmd", approveScript, "--notify-cmd", notifyScript, "--no-dev", "--workers", "0", "--max-expeditions", "1", ws.RepoPath)`
- amadeus: `ws.RunAmadeus(t, ctx, "check", "--approve-cmd", approveScript, "--notify-cmd", notifyScript, ws.RepoPath)`

**IMPORTANT:** Do NOT use `RunSightjackScan`/`RunPaintressExpedition`/`RunAmadeusCheck` — they hardcode `--auto-approve` which conflicts with `--approve-cmd`. Use the generic `RunSightjack`/`RunPaintress`/`RunAmadeus` helpers directly.

**Verification:**
```bash
cd /Users/nino/tap/sightjack && go test -tags scenario ./tests/scenario/ -run TestScenario_ApproveCmdPath -count=1 -v -timeout=180s
cd /Users/nino/tap/paintress && go test -tags scenario ./tests/scenario/ -run TestScenario_ApproveCmdPath -count=1 -v -timeout=180s
cd /Users/nino/tap/amadeus && go test -tags scenario ./tests/scenario/ -run TestScenario_ApproveCmdPath -count=1 -v -timeout=180s
```

**Commit (per repo):**
```
sightjack: tests: add approve-cmd/notify-cmd scenario test [BEHAVIORAL]
paintress: tests: add approve-cmd/notify-cmd scenario test [BEHAVIORAL]
amadeus: tests: add approve-cmd/notify-cmd scenario test [BEHAVIORAL]
```

---

## Task 7: SJ-2/PT-2 — Replace injection with real routing in L2 [BEHAVIORAL]

**Files:**
- Modify: `sightjack/tests/scenario/small_test.go`
- Modify: `paintress/tests/scenario/small_test.go`

**What:**
L2 の inbox 直接注入を、実ツール実行 + phonewave 配送に置換。

sightjack L2:
- 現状: 手動で feedback/convergence を inject
- 変更: amadeus 経由で feedback を生成させ、phonewave 配送で受領

paintress L2:
- 現状: 手動で spec を inject
- 変更: sightjack 経由で spec を生成させ、phonewave 配送で受領

**Implementation:**
sightjack small_test.go: inject → `ws.RunAmadeusCheck(t, ctx)` + `ws.WaitForDMail` に置換 (Task 0 で追加済み helper 使用)
paintress small_test.go: inject → `ws.RunSightjackScan(t, ctx)` + `ws.WaitForDMail` に置換 (Task 0 で追加済み helper 使用)

**Verification:**
```bash
cd /Users/nino/tap/sightjack && go test -tags scenario ./tests/scenario/ -run TestScenario_L2 -count=1 -v -timeout=180s
cd /Users/nino/tap/paintress && go test -tags scenario ./tests/scenario/ -run TestScenario_L2 -count=1 -v -timeout=180s
```

**Commit (per repo):**
```
sightjack: tests: replace injection with real routing in L2 [BEHAVIORAL]
paintress: tests: replace injection with real routing in L2 [BEHAVIORAL]
```

---

## Task 8: PW-3 — Daemon restart with downstream tool integration [BEHAVIORAL]

**Files:**
- Modify: `phonewave/tests/scenario/hard_test.go` (existing restart phase)

**What:**
phonewave daemon 再起動後の未配送メールが downstream ツール (paintress) で実際に処理されることを確認。
現状は outbox → inbox 配送のみ確認。downstream ツール実行を追加。

**Implementation:**
既存の hard_test.go Phase 2 (restart) の後に (Task 0 で追加済み `RunPaintressExpedition` helper 使用):
```go
// After restart, run paintress to process the delayed spec
err = ws.RunPaintressExpedition(t, ctx)
// Verify report generation after restart recovery
reportPath := ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)
obs.AssertDMailKind(reportPath, "report")
```

**Verification:**
```bash
cd /Users/nino/tap/phonewave && go test -tags scenario ./tests/scenario/ -run TestScenario_L4 -count=1 -v -timeout=600s
```

**Commit:**
```
phonewave: tests: add downstream tool verification after daemon restart [BEHAVIORAL]
```

---

## Task 9: T2-2 — go-expect limited introduction [BEHAVIORAL]

**Files:**
- Modify: `sightjack/tests/scenario/middle_test.go`
- Add go-expect dependency if not already in phonewave go.mod

**What:**
sightjack L3 で interactive path (wave selection) を PTY 経由でテスト。
`--auto-approve` を使わず、go-expect で対話応答を注入。

**Implementation:**
go-expect は既に sightjack の go.mod にある。scenario test で使用:
```go
// L3 variant: interactive approval via go-expect
func TestScenario_L3_Interactive(t *testing.T) {
    // ... setup ...
    c, err := expect.NewConsole(expect.WithDefaultTimeout(15*time.Second))
    // ... PTY setup, SIGHTJACK_TTY override ...
}
```

**Note:** L3 のみ。L1/L2 は `--auto-approve` のまま。

**Verification:**
```bash
cd /Users/nino/tap/sightjack && go test -tags scenario ./tests/scenario/ -run TestScenario_L3_Interactive -count=1 -v -timeout=300s
```

**Commit:**
```
sightjack: tests: add go-expect interactive path in L3 scenario [BEHAVIORAL]
```

---

## Execution Order

```
Task 0 (harness helpers)               ─── Phase 0: FIRST (structural, all repos)
                                         │
Task 1 (phonewave WaitForClosedLoop)  ─┐
Task 2 (phonewave convergence contract) ─┤── Phase A: parallel (phonewave only)
                                         │
Task 3 (sightjack L1 closed loop)      ─┤
Task 4 (paintress L1 closed loop)      ─┤── Phase B: parallel (3 repos, depends on Task 0)
Task 5 (amadeus L1 closed loop)        ─┘
                                         │
Task 6 (approve-cmd tests, 3 repos)    ─── Phase C: after Phase B
                                         │
Task 7 (real routing L2)               ─── Phase D: after Phase C
                                         │
Task 8 (PW-3 restart downstream)       ─┤
Task 9 (go-expect interactive)         ─┘── Phase E: after Phase D
```

---

## Definition of Done

- [x] phonewave L1: WaitForClosedLoop 使用
- [x] phonewave L3: convergence route 契約化 (sleep → assert)
- [x] sightjack L1: spec→report→feedback 完走
- [x] paintress L1: report→feedback→inbox 到達
- [x] amadeus L1: WaitForClosedLoop 使用
- [x] 3 tools: --approve-cmd/--notify-cmd scenario test
- [x] sightjack/paintress L2: 実経路化
- [x] phonewave L4: restart後の downstream ツール確認
- [x] sightjack L3: go-expect interactive path
- [x] 全 repo で `go test -tags scenario ./tests/scenario/` ALL PASS
