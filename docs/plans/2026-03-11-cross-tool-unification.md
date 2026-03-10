# Cross-Tool Unification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix runtime bugs (`--verbose` flag, SKILL.md routing) + unify doctor, update, and smoke-test across phonewave, sightjack, paintress, amadeus.

**Architecture:** Each tool lives in its own Git repo. All share hexagonal architecture (cmd -> usecase -> port <- session/domain). Changes are branch-per-tool, one PR each, squash-merged.

**Tech Stack:** Go 1.26, Cobra CLI, go-selfupdate, GitHub Actions

**Repos (module paths):**
- phonewave: `github.com/hironow/phonewave`
- sightjack: `github.com/hironow/sightjack`
- paintress: `github.com/hironow/paintress`
- amadeus: `github.com/hironow/amadeus`

**CRITICAL CWD RULE:** Every Bash invocation MUST use absolute paths. The shell does NOT persist `cd` between calls. Always: `cd /absolute/path/to/tool && command`.

---

## Task 0: Create feature branches (ALL tools first)

Before any code changes, create branches on all 4 tools to avoid committing to main.

**Step 1: Create branches**

```bash
cd <phonewave-root> && git checkout -b fix/cross-tool-unification
cd <sightjack-root> && git checkout -b fix/cross-tool-unification
cd <paintress-root> && git checkout -b fix/cross-tool-unification
cd <amadeus-root> && git checkout -b fix/cross-tool-unification
```

**Step 2: Verify all on correct branch**

```bash
cd <phonewave-root> && git branch --show-current
cd <sightjack-root> && git branch --show-current
cd <paintress-root> && git branch --show-current
cd <amadeus-root> && git branch --show-current
```

Expected: All print `fix/cross-tool-unification`

---

## Task 1: Bug Fix — Add `--verbose` to Claude CLI stream-json invocations (ALL 3 tools)

The Claude CLI requires `--verbose` when combining `--print` with `--output-format=stream-json`. Without it, stream-json output is empty/broken. This affects ALL tools that invoke Claude programmatically: amadeus, sightjack, and paintress.

### Task 1a: amadeus `--verbose`

**Files:**
- Modify: `amadeus/internal/session/claude.go:32-38`

**Step 1: Add `--verbose` to command construction**

In `amadeus/internal/session/claude.go`, the `defaultClaudeRunner.Run()` method builds the command. Add `"--verbose"` to the args:

```go
cmd := platform.NewShellCmd(ctx, claudeCmd,
    "--model", model,
    "--output-format", "stream-json",
    "--verbose",
    "--allowedTools", strings.Join(DivergenceMeterAllowedTools, ","),
    "--dangerously-skip-permissions",
    "--print",
)
```

**Step 2: Run tests**

```bash
cd <amadeus-root> && just test
```

Expected: PASS

**Step 3: Commit**

```bash
cd <amadeus-root> && git add internal/session/claude.go && git commit -m "fix: add --verbose to Claude CLI stream-json invocation

Claude CLI requires --verbose when combining --print with --output-format=stream-json.
Without it, stream-json output is empty."
```

### Task 1b: sightjack `--verbose`

**Files:**
- Modify: `sightjack/internal/session/claude.go:96`

**Step 1: Add `--verbose` to args construction**

In `sightjack/internal/session/claude.go`, find the line:
```go
args = append(args, "--output-format", "stream-json")
```

Change to:
```go
args = append(args, "--output-format", "stream-json", "--verbose")
```

**Step 2: Update test expectations**

In `sightjack/internal/session/claude_test.go`, the `TestRunClaudeOnce_ArgsWithModel` test checks exact args. Add `"--verbose"` to the expected slice after `"stream-json"`.

Similarly update `TestRunClaudeOnce_DefaultModel` if it checks args.

**Step 3: Run tests**

```bash
cd <sightjack-root> && just test
```

Expected: PASS

**Step 4: Commit**

```bash
cd <sightjack-root> && git add internal/session/claude.go internal/session/claude_test.go && git commit -m "fix: add --verbose to Claude CLI stream-json invocation"
```

### Task 1c: paintress `--verbose`

**Files:**
- Modify: `paintress/internal/session/expedition.go:201-206`

**Step 1: Add `--verbose` to command construction**

In `paintress/internal/session/expedition.go`, find:
```go
cmd := newCmd(expCtx, claudeCmd,
    "--model", model,
    "--output-format", "stream-json",
    "--dangerously-skip-permissions",
    "--print",
    "-p", prompt,
)
```

Add `"--verbose"` after `"stream-json"`:
```go
cmd := newCmd(expCtx, claudeCmd,
    "--model", model,
    "--output-format", "stream-json",
    "--verbose",
    "--dangerously-skip-permissions",
    "--print",
    "-p", prompt,
)
```

**Step 2: Check for other Claude invocations in paintress**

Search for `--output-format` in paintress session code. If `review.go` or `review_loop.go` also uses `--print --output-format stream-json`, add `--verbose` there too.

**Step 3: Run tests**

```bash
cd <paintress-root> && just test
```

Expected: PASS

**Step 4: Commit**

```bash
cd <paintress-root> && git add internal/session/expedition.go && git commit -m "fix: add --verbose to Claude CLI stream-json invocation"
```

---

## Task 2: Bug Fix — paintress SKILL.md + existing state regeneration

The code in `feedback.go` produces `implementation-feedback` D-Mails via `domain.NewEscalationDMail()`, but the SKILL.md template only declares `produces: [report]`. This causes phonewave to have no route for escalation messages.

**Important:** Fixing the template alone is NOT sufficient. Existing `.expedition/skills/` directories have the old SKILL.md. The `init` command must regenerate them.

**Files:**
- Modify: `paintress/internal/platform/templates/skills/dmail-sendable/SKILL.md`

**Step 1: Update SKILL.md template**

In `paintress/internal/platform/templates/skills/dmail-sendable/SKILL.md`, add `implementation-feedback` to produces:

```yaml
---
name: dmail-sendable
description: Produces D-Mail report messages to outbox/ after expedition completion.
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: report
    - kind: implementation-feedback
---

D-Mail send capability for paintress.
```

**Step 2: Verify `init` regenerates SKILL.md**

The `InitExpeditionDir` function (or equivalent) should overwrite SKILL.md from the embedded template on every `init` call. Verify this by reading the init code — it should use `go:embed` and always write the template. If it only writes when missing, it needs to be updated to always write (like amadeus's `InitGateDir` which compares and overwrites on change).

**Step 3: Run tests**

```bash
cd <paintress-root> && just test
```

Expected: PASS

**Step 4: Commit**

```bash
cd <paintress-root> && git add internal/platform/templates/skills/dmail-sendable/SKILL.md && git commit -m "fix: declare implementation-feedback in SKILL.md produces

Code in feedback.go produces implementation-feedback D-Mails via
NewEscalationDMail(), but SKILL.md only declared report. This caused
phonewave to have no route for escalation messages.

Users must run 'paintress init --force' to regenerate existing SKILL.md files."
```

---

## Task 3: Add `--verbose` to doctor inference check (3 tools)

The doctor inference check (`claude --print --output-format text --max-turns 1 "1+1="`) also needs `--verbose` for Claude CLI compatibility.

**Files:**
- Modify: `sightjack/internal/session/doctor.go` (line ~360)
- Modify: `paintress/internal/session/doctor.go` (line ~139)
- Modify: `amadeus/internal/cmd/doctor_checks.go` (line ~377)

**Step 1: Update inference command in all 3 tools**

Find each line like:
```go
inferCmd := <cmdConstructor>(inferCtx, claudeCmd, "--print", "--output-format", "text", "--max-turns", "1", "1+1=")
```

Add `"--verbose"` after `claudeCmd`:
```go
inferCmd := <cmdConstructor>(inferCtx, claudeCmd, "--verbose", "--print", "--output-format", "text", "--max-turns", "1", "1+1=")
```

Where `<cmdConstructor>` is:
- sightjack: `newCmd`
- paintress: `makeShellCmd`
- amadeus: `newShellCmd`

**Step 2: Run tests for each**

```bash
cd <sightjack-root> && just test
cd <paintress-root> && just test
cd <amadeus-root> && just test
```

Expected: All PASS

**Step 3: Commit each**

```bash
cd <sightjack-root> && git add internal/session/doctor.go && git commit -m "fix: add --verbose to doctor inference check for Claude CLI compatibility"
cd <paintress-root> && git add internal/session/doctor.go && git commit -m "fix: add --verbose to doctor inference check for Claude CLI compatibility"
cd <amadeus-root> && git add internal/cmd/doctor_checks.go && git commit -m "fix: add --verbose to doctor inference check for Claude CLI compatibility"
```

---

## Task 4: Standardize deprecated `kind: feedback` doctor hint

Each tool gives a different migration hint. Standardize the format while keeping tool-appropriate migration targets:
- sightjack (designer): `design-feedback`
- paintress (implementer): `implementation-feedback`
- amadeus (verifier): `design-feedback` or `implementation-feedback`

**Files:**
- Modify: `sightjack/internal/session/doctor.go` (deprecated kind hint)
- Modify: `paintress/internal/session/doctor.go` (deprecated kind hint)
- Modify: `amadeus/internal/cmd/doctor_checks.go` (deprecated kind hint)

**Step 1: Standardize hint format in each tool**

Use consistent format: `"deprecated kind 'feedback'; migrate to '<kind>' (run '<tool> init --force' to regenerate SKILL.md)"`.

Find and update the hint string in each tool's doctor check.

**Step 2: Update any test assertions that match on the old hint**

Check each tool's doctor tests for assertions on the deprecated feedback hint string. Update to match new format.

**Step 3: Run tests**

```bash
cd <sightjack-root> && just test
cd <paintress-root> && just test
cd <amadeus-root> && just test
```

Expected: All PASS

**Step 4: Commit each**

```bash
cd <sightjack-root> && git add internal/session/doctor.go && git commit -m "tidy: standardize deprecated feedback kind hint in doctor"
cd <paintress-root> && git add internal/session/doctor.go && git commit -m "tidy: standardize deprecated feedback kind hint in doctor"
cd <amadeus-root> && git add internal/cmd/doctor_checks.go && git commit -m "tidy: standardize deprecated feedback kind hint in doctor"
```

---

## Task 5: Add missing doctor tests for sightjack

sightjack has `doctor_mcp_test.go` and `doctor_hint_test.go` but no comprehensive `doctor_test.go`. Use paintress (826 lines) as reference.

**Files:**
- Create: `sightjack/internal/session/doctor_test.go`

**Step 1: Write doctor tests**

Create `sightjack/internal/session/doctor_test.go` (external test package `session_test`) with tests covering:

1. `TestCheckStateDir_Writable` — writable dir returns OK
2. `TestCheckStateDir_NotExist` — non-existent dir returns FAIL
3. `TestCheckSkills_ValidSkillMD` — valid SKILL.md with dmail-schema-version passes
4. `TestCheckSkills_DeprecatedFeedbackKind` — old `kind: feedback` detected as FAIL
5. `TestCheckSkills_MissingSkillMD` — missing SKILL.md returns FAIL
6. `TestCheckEventStore_ValidJSONL` — valid event files pass
7. `TestCheckEventStore_EmptyDir` — empty events dir passes (no events is OK)
8. `TestRunDoctor_HealthyState` — full happy path with fixtures

Pattern: use `t.TempDir()` for isolation, write fixture files, call exported check functions (`session.CheckStateDir`, `session.CheckSkills`, etc.).

**Step 2: Run tests**

```bash
cd <sightjack-root> && go test ./internal/session/ -run "TestCheck|TestRunDoctor" -v
```

Expected: All PASS

**Step 3: Commit**

```bash
cd <sightjack-root> && git add internal/session/doctor_test.go && git commit -m "test: add doctor check unit tests for sightjack"
```

---

## Task 6: Unify smoke-test.yaml null check (3 tools)

phonewave has robust `PREVIOUS` null handling (jq `// ""` fallback + `'null'` string check). Other 3 tools only check for empty string. When a repo has only 1 release, `jq '.[1].tagName'` returns `null` (literal string), which passes the `!= ''` check and causes the download step to fail.

**Files:**
- Modify: `sightjack/.github/workflows/smoke-test.yaml`
- Modify: `paintress/.github/workflows/smoke-test.yaml`
- Modify: `amadeus/.github/workflows/smoke-test.yaml`

**Step 1: Update jq query**

In each tool, find the `PREVIOUS=` line (~line 61):
```yaml
PREVIOUS=$(gh release list --repo "$REPO_FULL" --limit 2 --json tagName -q '.[1].tagName')
```

Change to:
```yaml
PREVIOUS=$(gh release list --repo "$REPO_FULL" --limit 2 --json tagName -q '.[1].tagName // ""')
```

**Step 2: Update skip condition**

Change:
```yaml
if: env.PREVIOUS == ''
```
To:
```yaml
if: env.PREVIOUS == '' || env.PREVIOUS == 'null'
```

**Step 3: Update all `PREVIOUS != ''` conditions**

For every subsequent step that checks `env.PREVIOUS != ''`, change to:
```yaml
if: env.PREVIOUS != '' && env.PREVIOUS != 'null'
```

This applies to 4 steps: "Download previous release", "Verify update --check", "Run self-update", "Verify updated version matches latest".

**Step 4: Verify YAML syntax**

```bash
cd <sightjack-root> && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/smoke-test.yaml'))" && echo "OK"
cd <paintress-root> && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/smoke-test.yaml'))" && echo "OK"
cd <amadeus-root> && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/smoke-test.yaml'))" && echo "OK"
```

Expected: All "OK"

**Step 5: Commit each**

```bash
cd <sightjack-root> && git add .github/workflows/smoke-test.yaml && git commit -m "fix: add null check to smoke-test PREVIOUS release detection"
cd <paintress-root> && git add .github/workflows/smoke-test.yaml && git commit -m "fix: add null check to smoke-test PREVIOUS release detection"
cd <amadeus-root> && git add .github/workflows/smoke-test.yaml && git commit -m "fix: add null check to smoke-test PREVIOUS release detection"
```

---

## Task 7: Rename paintress `makeShellCmd` to `newShellCmd`

Standardize command constructor naming across tools. sightjack uses `newCmd`, amadeus uses `newShellCmd`, paintress uses `makeShellCmd`. Rename paintress to `newShellCmd` for consistency with amadeus (most descriptive name).

**Files:**
- Modify: `paintress/internal/session/doctor.go`
- Modify: `paintress/internal/session/doctor_test.go` (if references exist)

**Step 1: Rename variable and all references**

In `paintress/internal/session/doctor.go`:
- Rename `var makeShellCmd` to `var newShellCmd`
- Rename the `OverrideShellCmd` or equivalent function's internal references
- Replace all `makeShellCmd(` calls with `newShellCmd(`

In test files, update any references to `makeShellCmd`.

**Step 2: Run tests**

```bash
cd <paintress-root> && just test
```

Expected: PASS

**Step 3: Commit**

```bash
cd <paintress-root> && git add internal/session/doctor.go internal/session/doctor_test.go && git commit -m "tidy: rename makeShellCmd to newShellCmd for cross-tool consistency"
```

---

## Task 8: Create PRs

**Step 1: Push and create PRs for each tool**

```bash
cd <sightjack-root> && git push -u origin fix/cross-tool-unification && gh pr create --title "fix: cross-tool doctor/update/smoke-test unification" --body "..."
cd <paintress-root> && git push -u origin fix/cross-tool-unification && gh pr create --title "fix: cross-tool doctor/update/smoke-test unification" --body "..."
cd <amadeus-root> && git push -u origin fix/cross-tool-unification && gh pr create --title "fix: cross-tool doctor/update/smoke-test unification" --body "..."
```

phonewave: only if there are changes beyond the already-open smoke-test PR #30.

PR body template:
```
## Summary
- Add --verbose to Claude CLI stream-json invocations
- Add --verbose to doctor inference check
- Standardize deprecated feedback kind hint
- Unify smoke-test.yaml null check
- [tool-specific items]

## Test plan
- [ ] `just test` passes
- [ ] `just lint` passes
- [ ] CI green
```

**Step 2: Verify CI passes, then squash merge**

---

## Execution Order and Dependencies

```
Task 0 (branches)         ─── MUST be first

Task 1a (amadeus --verbose)  ─┐
Task 1b (sightjack --verbose) ├── independent bug fixes, parallelizable
Task 1c (paintress --verbose) ─┘

Task 2 (paintress SKILL.md)  ─── independent

Task 3 (doctor --verbose)    ─── independent (different code paths from Task 1)
Task 4 (hint unification)    ─── independent
Task 5 (sightjack tests)     ─── independent
Task 6 (smoke-test null)     ─── independent
Task 7 (constructor rename)  ─── independent

Task 8 (PRs)                 ─── MUST be last
```

Tasks 1-2 are highest priority (runtime bugs). Tasks 3-7 are quality/consistency.
