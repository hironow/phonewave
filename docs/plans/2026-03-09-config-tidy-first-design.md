# Config System Tidy-First Refactoring

**Date:** 2026-03-09
**Scope:** paintress, sightjack, amadeus (phonewave best-effort)
**Status:** Approved

## Problem

1. Config structs lack type-level distinction between user-settable and computed fields
2. CLI commands bypass config.yaml by hardcoding constants (e.g. `platform.DefaultClaudeCmd`)
3. No semgrep enforcement prevents config bypass or computed field mutation
4. sightjack's `strictness.estimated` (LLM-computed) is writable via `config set`

## Design Decisions

### 1. Type-Level Separation: UserConfig / ComputedConfig

All 3 tools adopt a unified struct pattern:

```go
type Config struct {
    UserConfig     `yaml:",inline"`
    ComputedConfig `yaml:",inline"`
}
```

- `UserConfig`: human-editable fields (lang, model, workers, etc.)
- `ComputedConfig`: system-written fields (sightjack: estimated strictness; others: empty)
- `yaml:",inline"` keeps YAML output flat (no nesting), preserving existing format

### 2. ComputedConfig Protection

`setConfigField()` explicitly rejects computed keys:

```go
case "strictness.estimated":
    return fmt.Errorf("key %q is computed (read-only): cannot be set manually", key)
```

### 3. Config Bypass Fix

Commands that reference hardcoded defaults (paintress doctor/issues, amadeus check)
load config with fallback to DefaultConfig() when file is absent.

### 4. Semgrep Rules (ERROR severity)

**Rule 1: `config-no-hardcoded-claude-cmd`**

- paintress: block `platform.DefaultClaudeCmd` outside cobra flag definitions
- amadeus: block `"claude"` literal in exec arguments

**Rule 2: `config-no-computed-field-in-set`**

- Block `cfg.Computed.` field assignment in setConfigField functions

**Rule 3: `config-no-direct-computed-write`**

- Block cmd-layer writes to ComputedConfig fields
- Exclude session-layer write functions (WriteEstimatedStrictness)

### 5. Per-Tool Changes

**sightjack** (has real ComputedConfig):

- Move `Strictness.Estimated` into ComputedConfig
- Add computed-key rejection to setConfigField
- WriteEstimatedStrictness writes via ComputedConfig

**paintress** (empty ComputedConfig):

- Split ProjectConfig into UserConfig + ComputedConfig + ProjectConfig(embed)
- doctor.go/issues.go: replace DefaultClaudeCmd with config-loaded value

**amadeus** (empty ComputedConfig):

- Split Config into UserConfig + ComputedConfig + Config(embed)
- check.go: replace hardcoded "claude" with config-loaded value
- Add claude_cmd field to UserConfig

### 6. Testing

- Each tool: `config set` on computed key returns error
- Each tool: config round-trip (write defaults, reload, verify all fields)
- Semgrep: `just lint` catches violations
