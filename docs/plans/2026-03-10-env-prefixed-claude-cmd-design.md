# Env-Prefixed claude_cmd Hardening Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix two runtime bugs and add E2E verification that env-prefixed `claude_cmd` values propagate correctly through subprocess execution.

**Architecture:** Defensive fixes in session/domain layers + fake-claude env logging + scenario L1 tests across sightjack, paintress, amadeus.

**Scope:** sightjack, paintress, amadeus (phonewave does not execute claude_cmd).

---

## Context

Users configure `claude_cmd` with env-prefix patterns like:

```yaml
claude_cmd: "CLAUDE_CONFIG_DIR=~/.claude-work-b ~/.local/bin/claude"
```

This string flows through `platform.ParseShellCommand` which extracts env vars, expands tildes, and constructs `exec.Cmd` with the correct `Env` slice. However:

1. No E2E tests verify env vars actually propagate to subprocesses
2. A runtime panic exists in paintress when `dev_cmd` is empty
3. Preflight error messages show raw cmdLine instead of resolved binary

## Bug Fix 1: paintress devserver panic

**Root cause:** `strings.Fields("")` returns empty slice, `parts[0]` panics.

**Fix (2 locations):**

### devserver.go — defensive check

```go
parts := strings.Fields(ds.cmd)
if len(parts) == 0 {
    return fmt.Errorf("dev_cmd is empty; set dev_cmd or enable no_dev")
}
ds.process = exec.CommandContext(ctx, parts[0], parts[1:]...)
```

### domain/config.go — validation

Add to `ValidateProjectConfig`:

```go
if !cfg.NoDev && cfg.DevCmd == "" {
    errs = append(errs, "dev_cmd must not be empty when no_dev is false")
}
```

## Bug Fix 2: preflight error message (all 3 tools)

**Root cause:** Error shows raw cmdLine including env vars, confusing users.

**Fix:** Show resolved binary name with original as context:

```go
_, resolved, _ := platform.ParseShellCommand(bin)
return fmt.Errorf("preflight: %s not found in PATH (from %q)", resolved, bin)
```

Applied to `internal/session/preflight.go` in sightjack, paintress, amadeus.

## Feature: fake-claude env logging

### Mechanism

All fake-claude binaries gain `FAKE_CLAUDE_ENV_LOG_DIR` support (same pattern as existing `FAKE_CLAUDE_PROMPT_LOG_DIR`):

- When `FAKE_CLAUDE_ENV_LOG_DIR` is set, every invocation writes a JSON file:
  ```
  /tmp/env-logs/env_001.json
  ```
- Content:
  ```json
  {"CLAUDE_CONFIG_DIR": "/expanded/path/.claude-work-b", "args": ["--version"]}
  ```
- Sequential numbering (same as prompt logging)
- No-op when env var is unset

### Files (4 fake-claude binaries)

- `sightjack/tests/e2e/fake-claude/main.go`
- `sightjack/tests/scenario/testdata/fake-claude/main.go`
- `paintress/tests/scenario/testdata/fake-claude/main.go`
- `amadeus/tests/scenario/testdata/fake-claude/main.go`

## Feature: scenario L1 env propagation test

### Test: `TestScenario_L1_EnvPrefixedClaudeCmd`

Added to each tool's scenario test suite.

**Flow:**

1. Harness builds fake-claude to `dist/fake-claude`
2. Creates temp dir for env logs (`FAKE_CLAUDE_ENV_LOG_DIR`)
3. Writes config with:
   ```yaml
   claude_cmd: "CLAUDE_CONFIG_DIR=/tmp/test-config /abs/path/dist/fake-claude"
   ```
4. Runs `doctor` command (exercises --version, mcp list, inference)
5. Reads env log files
6. Asserts `CLAUDE_CONFIG_DIR == "/tmp/test-config"` in every log entry

### Verification matrix

| Check | What it exercises |
|-------|-------------------|
| preflight | `LookPathShell` resolves env-prefixed cmd |
| doctor --version | `NewShellCmd` passes env vars to subprocess |
| doctor mcp list | Same, different args |
| doctor inference | `--print` execution with env propagation |

## Out of scope

- Worktree-internal env propagation (L3+ scenario scope)
- phonewave (no claude_cmd)
- Docker E2E env propagation (existing E2E uses simple `claude` binary name)
