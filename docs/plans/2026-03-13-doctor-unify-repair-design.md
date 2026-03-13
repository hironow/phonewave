# Doctor Unification + --repair Design

**Date:** 2026-03-13
**Status:** Approved

## Goal

4 CLI tools (sightjack, amadeus, paintress, phonewave) の doctor コマンドにおける:
1. Claude 関連チェックの統一（チェック名、ステータス、ヒント文言、カスケードロジック）
2. `--repair` フラグの全ツール展開（skills-ref install, SKILL.md 再生成, 状態ディレクトリ作成, stale PID cleanup）
3. testcontainer ベースの e2e テスト追加（TDD Red から）

## Architecture

### Status 統一

全3ツール (sightjack, amadeus, paintress) の `domain.CheckStatus` に `CheckFixed` を追加:

```go
const (
    CheckOK    CheckStatus = "ok"
    CheckFail  CheckStatus = "fail"
    CheckWarn  CheckStatus = "warn"
    CheckSkip  CheckStatus = "skip"
    CheckFixed CheckStatus = "fixed"  // NEW
)
```

phonewave は既に `"fixed"` severity を持つ。

### Claude 関連チェック統一仕様

**Check names (lowercase kebab):** `claude-auth`, `linear-mcp`, `claude-inference`, `context-budget`

**Status rules:**

| Condition | claude-auth | linear-mcp | claude-inference | context-budget |
|-----------|-------------|------------|------------------|----------------|
| claude binary not found | SKIP | SKIP | SKIP | SKIP |
| mcp list failed (auth) | WARN | SKIP | SKIP | SKIP |
| mcp list OK, linear disconnected | OK | WARN | - | - |
| inference failed | OK | - | WARN | SKIP |
| inference OK, budget exceeded | OK | - | OK | WARN |
| All healthy | OK | OK | OK | OK |

**Cascade flow:**

```
claude binary found?
+- NO -> SKIP all 4
+- YES
   +- claude mcp list (10s timeout)
      +- FAIL -> claude-auth: WARN
      |         linear-mcp: SKIP "skipped (auth failed)"
      |         claude-inference: SKIP "skipped (auth failed)"
      |         context-budget: SKIP "skipped (auth failed)"
      +- OK -> claude-auth: OK
              +- linear in mcp output? -> OK or WARN
              +- claude --print 1+1= (3m timeout)
                 +- FAIL/wrong -> claude-inference: WARN
                 |               context-budget: SKIP "skipped (inference failed)"
                 +- OK -> claude-inference: OK
                         context-budget: from stream output
```

**amadeus:** `claude-login` check abolished. Unified to `claude mcp list` based `claude-auth`.

**Hint text (identical across all tools):**

- claude-auth WARN: `run "claude login" to authenticate`
- linear-mcp WARN: `run "claude mcp add --transport http --scope project linear https://mcp.linear.app/mcp" in your project root`
- claude-inference WARN: `"signal: killed" = CLI startup too slow (timeout 3m); "nested session" = CLAUDECODE env var leaked; otherwise check API key, quota, and model access`
- claude-inference WARN (unexpected): `model returned unexpected output; check model access and API quota`

### --repair Target Items

| # | Check | Condition | Repair Action | Post-repair Status |
|---|-------|-----------|---------------|--------------------|
| A | skills-ref | uv present, skills-ref absent, no submodule | `uv tool install skills-ref` | CheckFixed |
| B | skills (SKILL.md) | SKILL.md missing or deprecated kind | Regenerate (init-equivalent logic) | CheckFixed |
| C | State directory | `.siren/` `.gate/` `.expedition/` missing | `mkdir -p` | CheckFixed |
| D | stale PID | PID file exists, process dead | Remove PID file | CheckFixed |

**skills-ref idempotency rule (established in phonewave):**

```
skills-ref on PATH? -> OK, return
uv on PATH? -> NO -> WARN, return
submodule available? -> YES -> OK (venv report), return
repair=true? -> uv tool install skills-ref -> CheckFixed or WARN
repair=false? -> WARN with hint
```

### Signature Changes

```go
// sightjack
func RunDoctor(ctx, configPath, baseDir, logger, repair bool) []domain.DoctorCheck

// amadeus
func runDoctor(ctx, configPath, repoRoot, logger, repair bool) []domain.DoctorCheck

// paintress
func RunDoctor(claudeCmd, continent, repair bool) []domain.DoctorCheck
```

### Injectable Functions

| Injectable | sightjack | amadeus | paintress | Purpose |
|-----------|:---------:|:-------:|:---------:|---------|
| lookPathFn | NEW | existing | existing | skills-ref/uv check |
| installSkillsRefFn | NEW | NEW | NEW | `uv tool install skills-ref` |
| findSkillsRefDirFn | NEW | NEW | NEW | Submodule search |
| generateSkillsFn | NEW | NEW | NEW | SKILL.md regeneration |

### Testcontainer E2E Tests

Each tool gets `tests/e2e/` with:

```
tests/e2e/
+-- main_test.go              # TestMain: image build & cache
+-- testdata/Dockerfile.test  # Alpine + tool binary
+-- doctor_repair_test.go     # --repair e2e tests
```

Build tag: `//go:build e2e`

sightjack/amadeus need `testcontainers-go v0.40.0` added to go.mod.

**Test scenarios (TDD Red-first, all tools):**

| Test | Given | When | Then |
|------|-------|------|------|
| TestDoctorRepair_StalePID | watch.pid with dead PID | `doctor --repair --json` | PID file removed, JSON has "fixed" |
| TestDoctorRepair_MissingStateDir | State dir not created | `doctor --repair --json` | Dir created, JSON has "fixed" |
| TestDoctorRepair_MissingSkillMD | init'd but SKILL.md deleted | `doctor --repair --json` | SKILL.md regenerated, JSON has "fixed" |
| TestDoctorRepair_NoRepairFlag | Same broken state | `doctor --json` (no repair) | WARN only, no modifications |

skills-ref install tested via injectable unit tests (not e2e — uv in container is costly).

### Checker Script

`/Users/nino/tap/scripts/check_doctor_consistency.sh` validates:

1. Check names unified across all tools
2. Status rules: auth/linear/inference failure = WARN
3. Hint text hash identical
4. Cascade: auth failure -> inference/budget SKIP
5. Timeout: inference = `3*time.Minute`
6. `CheckFixed` exists in all tool domains
7. `--repair` flag exists in all cmd/doctor.go
8. Injectable functions exist in all tools

### Execution Order

1. Add `CheckFixed` to all 3 tool domains
2. Create checker script (define criteria first)
3. sightjack: e2e RED -> claude unification + --repair -> e2e GREEN
4. amadeus: e2e RED -> claude unification + claude-login removal + --repair -> e2e GREEN
5. paintress: e2e RED -> claude unification + --repair -> e2e GREEN
6. Run checker script -> all PASS
7. All tools: go vet + go test ./... + go test -tags e2e

### Tool-Specific Notes

| Tool | State Dir | SKILL.md Location | Daemon |
|------|-----------|-------------------|--------|
| sightjack | `.siren/` | `.siren/skills/dmail-{sendable,readable}/SKILL.md` | YES |
| amadeus | `.gate/` | `.gate/skills/dmail-{sendable,readable}/SKILL.md` | YES |
| paintress | `.expedition/` + subdirs | `.expedition/skills/dmail-{sendable,readable}/SKILL.md` | YES |
| phonewave | `.phonewave/` | N/A (orchestrator) | YES |
