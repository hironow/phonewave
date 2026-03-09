# Config Field Unification Design

**Date:** 2026-03-10
**Scope:** sightjack (primary), paintress (minor)
**Status:** Approved

## Problem

1. sightjack uses nested `assistant.{command, model, timeout_sec}` while paintress/amadeus use flat `claude_cmd`, `model`, `timeout_sec`
2. sightjack `DefaultConfig()` leaves `Assistant.Command` empty; fallback `"claude"` is hardcoded in `LoadConfig`, `review_gate.go`, `doctor.go`
3. paintress `IssueTrackerConfig.Project` has `omitempty` but sightjack does not — minor inconsistency
4. sightjack lacks a semgrep rule to prevent hardcoded `"claude"` literals

## Design Decisions

### 1. Flatten sightjack AIAssistantConfig

Remove `AIAssistantConfig` struct. Promote fields to `Config` top-level:

```go
// Before
type Config struct {
    Assistant AIAssistantConfig `yaml:"assistant"`
    // ...
}

// After
type Config struct {
    ClaudeCmd  string `yaml:"claude_cmd,omitempty"`
    Model      string `yaml:"model,omitempty"`
    TimeoutSec int    `yaml:"timeout_sec,omitempty"`
    // ...
}
```

CLI key mapping:
- `assistant.command` → `claude_cmd`
- `assistant.model` → `model`
- `assistant.timeout_sec` → `timeout_sec`

### 2. DefaultConfig gets real defaults

```go
func DefaultConfig() Config {
    return Config{
        ClaudeCmd:  "claude",
        Model:      "opus",
        TimeoutSec: 300,
        // ...
    }
}
```

Remove all LoadConfig fallback assignments and hardcoded `"claude"` in review_gate.go, doctor.go.

### 3. Tracker unification (sightjack ↔ paintress)

Both already share `IssueTrackerConfig` with `Team` and `Project`.
- Remove `omitempty` from paintress `Project` field to match sightjack
- sightjack keeps `Cycle` as tool-specific field (paintress does not use cycles)

### 4. Semgrep rule for sightjack

Add `config-no-hardcoded-claude-literal` rule (same pattern as amadeus) to block `"claude"` string literals in exec arguments outside test files.

### 5. Config set backward compatibility

Accept both old and new keys during transition:

```go
case "claude_cmd", "assistant.command":
    cfg.ClaudeCmd = value
case "model", "assistant.model":
    cfg.Model = value
case "timeout_sec", "assistant.timeout_sec":
    cfg.TimeoutSec = value
```

CLI help text shows only new keys. Old keys work silently.

### 6. YAML backward compatibility

Since `yaml:",inline"` is not used here, old `assistant:` YAML blocks will be silently ignored after the migration. Users running `sightjack init` will get the new flat format.

This is acceptable: we do not need backward compatibility per project CLAUDE.md.

## Files Changed

**sightjack (primary):**
- `internal/domain/config.go` — Remove AIAssistantConfig, add flat fields, update DefaultConfig
- `internal/domain/config_test.go` — Update field references
- `internal/session/config.go` — Update setConfigField keys, remove LoadConfig fallbacks
- `internal/session/config_test.go` — Update all `.Assistant.X` references
- `internal/session/claude.go` — `cfg.Assistant.Command` → `cfg.ClaudeCmd` etc.
- `internal/session/claude_test.go` — Update config construction
- `internal/session/doctor.go` — Remove hardcoded claudeName, use config
- `internal/session/review_gate.go` — Remove hardcoded fallback
- `internal/session/phases.go` — Update if referencing Assistant
- `internal/cmd/config.go` — Update help text
- `internal/cmd/run.go` — `cfg.Assistant.Command` → `cfg.ClaudeCmd`
- `.semgrep/layers.yaml` — Add config-no-hardcoded-claude-literal
- ~10 test files — Mechanical `.Assistant.X` → flat field replacement

**paintress (minor):**
- `internal/domain/config.go` — Remove `omitempty` from `IssueTrackerConfig.Project`

## Testing

- sightjack: `config set claude_cmd cc-p` round-trip test
- sightjack: `config set assistant.command cc-p` backward compat test
- sightjack: DefaultConfig test verifies ClaudeCmd, Model, TimeoutSec have defaults
- sightjack: semgrep catches `"claude"` literal in non-test code
- paintress: existing tests still pass (omitempty removal is cosmetic)
