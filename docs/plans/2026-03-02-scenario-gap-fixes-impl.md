# Scenario Gap Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** D-Mail closed loop の 3 つの Gap (sightjack feedback route, phonewave fixture, sightjack --auto-approve) を修正し、シナリオテストで検証する。

**Architecture:** sightjack SKILL.md のルーティング宣言修正、phonewave fixture の同期、sightjack session.go の interactive loop 拡張。

**Tech Stack:** Go, cobra CLI, YAML frontmatter D-Mail, fsnotify-based routing

**Reference:** `/Users/nino/tap/phonewave/docs/plans/2026-03-02-scenario-gap-fixes-design.md`

---

## Task Dependencies

```
Task 1 (SKILL.md) ─────────────────────> Task 4 (verification)
Task 2 (fixture) ──────────────────────> Task 4
Task 3 (auto-approve) ─> Task 3b ─────> Task 4
```

- Task 1, 2, 3 are independent (parallel-safe)
- Task 3b depends on Task 3
- Task 4 depends on all prior tasks

---

## Task 1: Add feedback to sightjack SKILL.md produces [BEHAVIORAL]

**Files:**
- Modify: `/Users/nino/tap/sightjack/templates/skills/dmail-sendable/SKILL.md`

**Step 1: Modify SKILL.md**

Current content:
```yaml
---
name: dmail-sendable
description: Declares outbound D-Mail kinds for phonewave routing discovery.
license: Apache-2.0
compatibility: Requires phonewave daemon or direct filesystem access.
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
    - kind: report
---

Sightjack D-Mail sendable skill.
```

Add `- kind: feedback` to produces:
```yaml
---
name: dmail-sendable
description: Declares outbound D-Mail kinds for phonewave routing discovery.
license: Apache-2.0
compatibility: Requires phonewave daemon or direct filesystem access.
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
    - kind: report
    - kind: feedback
---

Sightjack D-Mail sendable skill.
```

**Step 2: Run sightjack unit tests**

```bash
cd /Users/nino/tap/sightjack && just test
```

Expected: all tests pass (SKILL.md is go:embed'd, parsed at runtime by phonewave, not by sightjack directly).

**Step 3: Commit**

```
sightjack: add feedback kind to dmail-sendable SKILL.md produces [BEHAVIORAL]
```

---

## Task 2: Update pt_expedition.txt fixtures + fake-claude default (all 3 repos) [BEHAVIORAL]

**Files:**
- Modify: `/Users/nino/tap/phonewave/tests/scenario/testdata/fixtures/minimal/pt_expedition.txt`
- Modify: `/Users/nino/tap/phonewave/tests/scenario/testdata/fixtures/small/pt_expedition.txt`
- Modify: `/Users/nino/tap/phonewave/tests/scenario/testdata/fixtures/middle/pt_expedition.txt`
- Modify: `/Users/nino/tap/phonewave/tests/scenario/testdata/fixtures/hard/pt_expedition.txt`
- Modify: `/Users/nino/tap/phonewave/tests/scenario/testdata/fake-claude/main.go`
- Modify: `/Users/nino/tap/sightjack/tests/scenario/testdata/fixtures/minimal/pt_expedition.txt`
- Modify: `/Users/nino/tap/sightjack/tests/scenario/testdata/fixtures/small/pt_expedition.txt`
- Modify: `/Users/nino/tap/sightjack/tests/scenario/testdata/fixtures/middle/pt_expedition.txt`
- Modify: `/Users/nino/tap/sightjack/tests/scenario/testdata/fixtures/hard/pt_expedition.txt`
- Modify: `/Users/nino/tap/sightjack/tests/scenario/testdata/fake-claude/main.go`
- Modify: `/Users/nino/tap/amadeus/tests/scenario/testdata/fixtures/minimal/pt_expedition.txt`
- Modify: `/Users/nino/tap/amadeus/tests/scenario/testdata/fixtures/small/pt_expedition.txt`
- Modify: `/Users/nino/tap/amadeus/tests/scenario/testdata/fixtures/middle/pt_expedition.txt`
- Modify: `/Users/nino/tap/amadeus/tests/scenario/testdata/fixtures/hard/pt_expedition.txt`
- Modify: `/Users/nino/tap/amadeus/tests/scenario/testdata/fake-claude/main.go`

NOTE: paintress は既に修正済み。phonewave, sightjack, amadeus の 3 repo が対象。

**Step 1: Update all 12 pt_expedition.txt fixtures (4 levels x 3 repos)**

Replace content of each file with the fixed format (matching paintress fixture):
```
I'll analyze the issues and work on them systematically.

## Issue Analysis

Looking at the assigned issues, I'll start with the highest priority item.

### Changes Made

1. Updated the configuration file
2. Fixed the validation logic
3. Added missing test coverage

All changes have been committed and pushed.

__EXPEDITION_REPORT__
issue_id: TEST-001
issue_title: Test action
mission_type: add_dod
branch: feat/test-001
pr_url: none
status: success
reason: completed
remaining_issues: 0
bugs_found: 0
bug_issues: none
insight: All tasks completed successfully
failure_type: none
__EXPEDITION_END__
```

**Step 2: Update fake-claude defaultPaintressResponse (3 repos)**

In each repo's `tests/scenario/testdata/fake-claude/main.go`, change `defaultPaintressResponse` to include the `__EXPEDITION_REPORT__` / `__EXPEDITION_END__` markers (same content as the fixture files):
- `/Users/nino/tap/phonewave/tests/scenario/testdata/fake-claude/main.go`
- `/Users/nino/tap/sightjack/tests/scenario/testdata/fake-claude/main.go`
- `/Users/nino/tap/amadeus/tests/scenario/testdata/fake-claude/main.go`

**Step 3: Run tests in all 3 repos**

```bash
cd /Users/nino/tap/phonewave && just test
cd /Users/nino/tap/sightjack && just test
cd /Users/nino/tap/amadeus && just test
```

Expected: all tests pass.

**Step 4: Commit (one per repo)**

```
phonewave: tests: fix pt_expedition fixtures with report markers [BEHAVIORAL]
sightjack: tests: fix pt_expedition fixtures with report markers [BEHAVIORAL]
amadeus: tests: fix pt_expedition fixtures with report markers [BEHAVIORAL]
```

---

## Task 3: Extend sightjack --auto-approve to cover interactive loop [BEHAVIORAL]

**Files:**
- Modify: `/Users/nino/tap/sightjack/internal/session/session.go` (functions: `selectPhase`, `approvalPhase`, `runInteractiveLoop`)

**Step 1: Add autoApprove parameter to selectPhase**

Current signature (line 189):
```go
func selectPhase(ctx context.Context, scanner *bufio.Scanner,
	scanResult *sightjack.ScanResult, cfg *sightjack.Config, available []sightjack.Wave, waves []sightjack.Wave,
	adrCount int, resumedAt *time.Time, shibitoShown bool,
	out io.Writer, loopSpan trace.Span, logger *sightjack.Logger) (sightjack.Wave, selectPhaseResult, bool) {
```

Add early return at the start of `selectPhase` body, before Navigator display:
```go
// Auto-select first available wave when --auto-approve is set.
if cfg.Gate.AutoApprove {
    if len(available) > 0 {
        logger.Info("Auto-selecting wave: %s", available[0].Title)
        return available[0], selectChosen, shibitoShown
    }
    return sightjack.Wave{}, selectQuit, shibitoShown
}
```

This skips `PromptWaveSelection` and all interactive display. `cfg` is already a parameter.

**Step 2: Add early return to approvalPhase**

Current signature (line 249):
```go
func approvalPhase(ctx context.Context, scanner *bufio.Scanner,
	cfg *sightjack.Config, scanDir string, selected sightjack.Wave, resolvedStrictness string,
	...
```

Add early return at the start of `approvalPhase` body, before the `for` loop:
```go
// Auto-approve when --auto-approve is set.
if cfg.Gate.AutoApprove {
    loopSpan.AddEvent("wave.auto_approved",
        trace.WithAttributes(
            attribute.String("wave.id", selected.ID),
            attribute.String("wave.cluster_name", selected.ClusterName),
        ),
    )
    recorder.Record(sightjack.EventWaveApproved, sightjack.WaveIdentityPayload{
        WaveID: selected.ID, ClusterName: selected.ClusterName,
    })
    if err := ComposeSpecification(store, selected); err != nil {
        logger.Warn("D-Mail specification failed (non-fatal): %v", err)
    } else {
        recorder.Record(sightjack.EventSpecificationSent, sightjack.WaveIdentityPayload{
            WaveID: selected.ID, ClusterName: selected.ClusterName,
        })
    }
    return selected, approvalApproved
}
```

**Step 3: Run sightjack unit tests**

```bash
cd /Users/nino/tap/sightjack && just test
```

Expected: all existing tests pass (auto-approve path is additive, existing manual path unchanged).

**Step 4: Commit**

```
sightjack: extend --auto-approve to cover wave selection and approval [BEHAVIORAL]
```

---

## Task 3b: Remove stdin hack from sightjack scenario harness [STRUCTURAL]

**Files:**
- Modify: `/Users/nino/tap/sightjack/tests/scenario/harness_test.go`

**Step 1: Simplify RunSightjackScan**

Current implementation (line 293-308):
```go
func (w *Workspace) RunSightjackScan(t *testing.T, ctx context.Context, extraArgs ...string) error {
	t.Helper()
	args := []string{"run", "--auto-approve"}
	args = append(args, extraArgs...)
	args = append(args, w.RepoPath)

	cmd := w.runToolCmd(ctx, "sightjack", args...)

	// Provide interactive input: select wave 1, approve all, quit.
	cmd.Stdin = strings.NewReader("1\na\nq\n")
	// ...
```

Remove stdin hack now that --auto-approve covers the interactive loop:
```go
func (w *Workspace) RunSightjackScan(t *testing.T, ctx context.Context, extraArgs ...string) error {
	t.Helper()
	args := []string{"run", "--auto-approve"}
	args = append(args, extraArgs...)
	args = append(args, w.RepoPath)

	cmd := w.runToolCmd(ctx, "sightjack", args...)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	// ...
```

Remove `strings` import if no longer needed.

**Step 2: Run sightjack scenario tests**

```bash
cd /Users/nino/tap/sightjack && just test-scenario-min
```

Expected: L1 passes without stdin input (--auto-approve now covers wave selection + approval).

**Step 3: Commit**

```
sightjack: tests: remove stdin hack from RunSightjackScan [STRUCTURAL]
```

---

## Task 4: Cross-tool verification [VERIFICATION]

**Step 1: Run sightjack scenario tests (all levels)**

```bash
cd /Users/nino/tap/sightjack && just test-scenario-all
```

Expected: L1-L4 all pass. ERR log `no route for kind="feedback"` should no longer appear (requires phonewave re-init in test workspace to pick up the updated SKILL.md).

**Step 2: Run phonewave scenario tests**

```bash
cd /Users/nino/tap/phonewave && just test-scenario-min
```

Expected: L1 passes (phonewave has scenario tests via `justfile`).

**Step 3: Run paintress + amadeus scenario tests**

```bash
cd /Users/nino/tap/paintress && just test-scenario-all
cd /Users/nino/tap/amadeus && just test-scenario-all
```

Expected: all pass (these tools are unaffected by the sightjack changes, but verify no regressions).

**Step 4: Verify feedback route in sightjack L1**

In the L1 test output, look for:
- No `ERR Deliver .siren/outbox/feedback-*: no route for kind="feedback"` log
- Successful wave auto-selection log: `Auto-selecting wave:`

---

## Summary

| Task | Repo | Type | Risk |
|------|------|------|------|
| 1: SKILL.md feedback | sightjack | BEHAVIORAL | Low — additive change |
| 2: pt_expedition fixtures | phonewave, sightjack, amadeus | BEHAVIORAL | Low — test-only files |
| 3: --auto-approve extension | sightjack | BEHAVIORAL | Medium — modifies interactive loop |
| 3b: Remove stdin hack | sightjack | STRUCTURAL | Low — test-only, depends on Task 3 |
| 4: Cross-tool verification | all | VERIFICATION | None |
