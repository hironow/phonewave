# Config Tidy-First Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Split config structs into UserConfig / ComputedConfig across paintress, sightjack, amadeus — protecting computed fields, fixing config bypass, and adding semgrep enforcement.

**Architecture:** Each tool's Config gets embedded UserConfig + ComputedConfig with `yaml:",inline"` to keep YAML flat. setConfigField rejects computed keys. Commands that bypass config.yaml get fixed to load config with fallback. Semgrep ERROR rules block future regressions.

**Tech Stack:** Go 1.26, cobra, gopkg.in/yaml.v3, semgrep

---

## Phase 1: sightjack — UserConfig / ComputedConfig (real computed field)

sightjack has a real ComputedConfig use case: `Strictness.Estimated` is LLM-computed and must be read-only via `config set`.

### Task 1: Test computed-key rejection in setConfigField

**Files:**

- Modify: `/Users/nino/tap/sightjack/internal/session/config_test.go`

**Step 1: Write the failing test**

```go
func TestSetConfigField_RejectsComputedKey(t *testing.T) {
 // given
 dir := t.TempDir()
 cfg := domain.DefaultConfig()
 data, _ := yaml.Marshal(cfg)
 cfgPath := filepath.Join(dir, ".siren", "config.yaml")
 os.MkdirAll(filepath.Dir(cfgPath), 0755)
 os.WriteFile(cfgPath, data, 0644)

 // when
 err := UpdateConfig(cfgPath, "strictness.estimated", "fog")

 // then
 if err == nil {
  t.Fatal("expected error for computed key")
 }
 if !strings.Contains(err.Error(), "computed") {
  t.Errorf("error should mention 'computed', got: %v", err)
 }
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/session/ -run TestSetConfigField_RejectsComputedKey -v -count=1`
Expected: FAIL — currently `strictness.estimated` is not handled in setConfigField, falls through to `default: unknown config key`

**Step 3: Add computed-key rejection to setConfigField**

In `/Users/nino/tap/sightjack/internal/session/config.go`, add case before the `default:` in setConfigField:

```go
case "strictness.estimated":
 return fmt.Errorf("key %q is computed (read-only): cannot be set manually", key)
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/session/ -run TestSetConfigField_RejectsComputedKey -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/session/config.go internal/session/config_test.go
git -C /Users/nino/tap/sightjack commit -m "feat: reject computed key strictness.estimated in config set"
```

---

### Task 2: Introduce UserConfig / ComputedConfig types for sightjack

**Files:**

- Modify: `/Users/nino/tap/sightjack/internal/domain/config.go`
- Modify: `/Users/nino/tap/sightjack/internal/domain/config_test.go`

**Step 1: Write the failing test — type-level separation**

In `/Users/nino/tap/sightjack/internal/domain/config_test.go`:

```go
func TestConfig_UserAndComputedEmbedding(t *testing.T) {
 // given
 cfg := DefaultConfig()

 // then — UserConfig fields accessible
 if cfg.Lang != "ja" {
  t.Errorf("UserConfig.Lang = %q, want ja", cfg.Lang)
 }
 if cfg.Strictness.Default != StrictnessFog {
  t.Errorf("UserConfig.Strictness.Default = %v, want fog", cfg.Strictness.Default)
 }

 // then — ComputedConfig accessible (empty by default)
 if cfg.Computed.EstimatedStrictness != nil {
  t.Errorf("ComputedConfig.EstimatedStrictness should be nil by default")
 }
}

func TestConfig_YAMLRoundTrip_FlatOutput(t *testing.T) {
 // given
 cfg := DefaultConfig()
 cfg.Computed.EstimatedStrictness = map[string]StrictnessLevel{
  "cluster-a": StrictnessHaze,
 }

 // when — marshal
 data, err := yaml.Marshal(cfg)
 if err != nil {
  t.Fatalf("marshal: %v", err)
 }

 // then — YAML should be flat (no "user:" or "computed:" wrapper keys)
 text := string(data)
 if strings.Contains(text, "user:") {
  t.Errorf("YAML should be flat, found 'user:' key:\n%s", text)
 }
 if strings.Contains(text, "computed:") {
  t.Errorf("YAML should be flat, found 'computed:' key:\n%s", text)
 }

 // when — unmarshal round-trip
 var loaded Config
 if err := yaml.Unmarshal(data, &loaded); err != nil {
  t.Fatalf("unmarshal: %v", err)
 }

 // then — values preserved
 if loaded.Lang != "ja" {
  t.Errorf("round-trip Lang = %q, want ja", loaded.Lang)
 }
 est, ok := loaded.Computed.EstimatedStrictness["cluster-a"]
 if !ok || est != StrictnessHaze {
  t.Errorf("round-trip EstimatedStrictness[cluster-a] = %v (ok=%v), want haze", est, ok)
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/domain/ -run 'TestConfig_(UserAndComputed|YAMLRoundTrip_Flat)' -v -count=1`
Expected: FAIL — `cfg.Computed` does not exist

**Step 3: Refactor Config struct with UserConfig / ComputedConfig**

In `/Users/nino/tap/sightjack/internal/domain/config.go`:

1. Create new `UserStrictnessConfig` that holds only `Default` and `Overrides` (user-settable):

```go
// UserStrictnessConfig holds user-settable strictness fields.
type UserStrictnessConfig struct {
 Default   StrictnessLevel            `yaml:"default"`
 Overrides map[string]StrictnessLevel `yaml:"overrides,omitempty"`
}
```

2. Create `ComputedConfig`:

```go
// ComputedConfig holds system-written fields that cannot be set via `config set`.
type ComputedConfig struct {
 EstimatedStrictness map[string]StrictnessLevel `yaml:"estimated,omitempty"`
}
```

3. Rename old `StrictnessConfig` fields and embed both into Config:

```go
type Config struct {
 Tracker      IssueTrackerConfig     `yaml:"tracker"`
 Scan         ScanConfig             `yaml:"scan"`
 Assistant    AIAssistantConfig      `yaml:"assistant"`
 Scribe       ScribeConfig           `yaml:"scribe"`
 Strictness   UserStrictnessConfig   `yaml:"strictness"`
 Retry        RetryConfig            `yaml:"retry"`
 Labels       LabelsConfig           `yaml:"labels"`
 Gate         GateConfig             `yaml:"gate"`
 DoDTemplates map[string]DoDTemplate `yaml:"dod_templates,omitempty"`
 Lang         string                 `yaml:"lang"`
 Computed     ComputedConfig         `yaml:"computed,omitempty"`
}
```

NOTE: We keep `Computed` as a named field (not inline) because sightjack has real computed data under a `computed:` YAML key. This differs from paintress/amadeus where ComputedConfig is empty and inlined. The `omitempty` ensures it disappears from YAML when empty.

4. Update `DefaultConfig()` — remove `Estimated` from Strictness init, `Computed` zero-value is fine.

5. Update `StrictnessConfig` references throughout:
   - Remove `Estimated` field from old `StrictnessConfig` (now `UserStrictnessConfig`)
   - `cfg.Strictness.Estimated` → `cfg.Computed.EstimatedStrictness`

**Step 4: Fix all compile errors**

Files that reference `cfg.Strictness.Estimated`:

- `internal/session/config.go` — `WriteEstimatedStrictness()` function: change `cfg.Strictness.Estimated = estimated` to `cfg.Computed.EstimatedStrictness = estimated`
- `internal/session/scan.go` or wherever estimated strictness is read: `cfg.Strictness.Estimated` → `cfg.Computed.EstimatedStrictness`
- `internal/domain/strictness.go` — `ResolveStrictness()` if it reads Estimated: update path

Search for all `Strictness.Estimated` references:

```bash
cd /Users/nino/tap/sightjack && grep -rn 'Strictness\.Estimated\|\.Estimated\b' internal/ --include='*.go' | grep -v _test.go
```

Update each reference.

**Step 5: Run all tests**

Run: `cd /Users/nino/tap/sightjack && go test ./... -count=1 -timeout=300s`
Expected: ALL PASS

**Step 6: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/domain/config.go internal/domain/config_test.go internal/session/config.go
# (add any other files touched in Step 4)
git -C /Users/nino/tap/sightjack commit -m "refactor: split Config into UserStrictnessConfig + ComputedConfig

[STRUCTURAL] Move Strictness.Estimated into ComputedConfig.EstimatedStrictness.
YAML output adds computed: section only when populated."
```

---

### Task 3: Update WriteEstimatedStrictness to use ComputedConfig path

**Files:**

- Modify: `/Users/nino/tap/sightjack/internal/session/config.go`
- Modify: `/Users/nino/tap/sightjack/internal/session/config_test.go`

**Step 1: Write failing test for WriteEstimatedStrictness round-trip**

```go
func TestWriteEstimatedStrictness_WritesToComputedConfig(t *testing.T) {
 // given
 dir := t.TempDir()
 cfgPath := filepath.Join(dir, "config.yaml")
 cfg := domain.DefaultConfig()
 data, _ := yaml.Marshal(cfg)
 os.WriteFile(cfgPath, data, 0644)

 estimated := map[string]domain.StrictnessLevel{
  "cluster-x": domain.StrictnessHaze,
 }

 // when
 err := WriteEstimatedStrictness(cfgPath, estimated)

 // then
 if err != nil {
  t.Fatalf("WriteEstimatedStrictness: %v", err)
 }
 loaded, loadErr := LoadConfig(cfgPath)
 if loadErr != nil {
  t.Fatalf("LoadConfig: %v", loadErr)
 }
 got, ok := loaded.Computed.EstimatedStrictness["cluster-x"]
 if !ok || got != domain.StrictnessHaze {
  t.Errorf("EstimatedStrictness[cluster-x] = %v (ok=%v), want haze", got, ok)
 }
 // user fields preserved
 if loaded.Lang != "ja" {
  t.Errorf("Lang should be preserved, got %q", loaded.Lang)
 }
}
```

**Step 2: Run test — should pass if Task 2 was correct, fail if WriteEstimatedStrictness path is wrong**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/session/ -run TestWriteEstimatedStrictness_WritesToComputedConfig -v -count=1`

**Step 3: Fix WriteEstimatedStrictness if needed**

Ensure it writes to `cfg.Computed.EstimatedStrictness` not the old `cfg.Strictness.Estimated`.

**Step 4: Run full test suite**

Run: `cd /Users/nino/tap/sightjack && go test ./... -count=1 -timeout=300s`
Expected: ALL PASS

**Step 5: Run lint**

Run: `cd /Users/nino/tap/sightjack && just lint`
Expected: CLEAN

**Step 6: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/session/config.go internal/session/config_test.go
git -C /Users/nino/tap/sightjack commit -m "test: verify WriteEstimatedStrictness uses ComputedConfig path"
```

---

## Phase 2: paintress — UserConfig / ComputedConfig (empty computed)

paintress has no computed fields today, but gets the type-level pattern for consistency and future-proofing.

### Task 4: Add ComputedConfig to paintress ProjectConfig

**Files:**

- Modify: `/Users/nino/tap/paintress/internal/domain/config.go`
- Modify: `/Users/nino/tap/paintress/internal/domain/config_test.go`

**Step 1: Write failing test**

In `/Users/nino/tap/paintress/internal/domain/config_test.go`:

```go
func TestProjectConfig_ComputedConfig_EmptyByDefault(t *testing.T) {
 // given/when
 cfg := DefaultProjectConfig()

 // then — ComputedConfig exists but is empty
 if cfg.Computed != (ComputedConfig{}) {
  t.Errorf("ComputedConfig should be zero-value by default, got %+v", cfg.Computed)
 }
}

func TestProjectConfig_YAMLRoundTrip_NoComputedKey(t *testing.T) {
 // given — default config with empty computed
 cfg := DefaultProjectConfig()

 // when
 data, err := yaml.Marshal(cfg)
 if err != nil {
  t.Fatalf("marshal: %v", err)
 }

 // then — no "computed" key in YAML output
 if strings.Contains(string(data), "computed") {
  t.Errorf("YAML should not contain 'computed' when empty:\n%s", string(data))
 }

 // round-trip
 var loaded ProjectConfig
 if err := yaml.Unmarshal(data, &loaded); err != nil {
  t.Fatalf("unmarshal: %v", err)
 }
 if loaded.Lang != cfg.Lang {
  t.Errorf("round-trip Lang = %q, want %q", loaded.Lang, cfg.Lang)
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/nino/tap/paintress && go test ./internal/domain/ -run 'TestProjectConfig_(ComputedConfig|YAMLRoundTrip_No)' -v -count=1`
Expected: FAIL — `cfg.Computed` and `ComputedConfig` do not exist

**Step 3: Add ComputedConfig to domain**

In `/Users/nino/tap/paintress/internal/domain/config.go`:

```go
// ComputedConfig holds system-written fields. Empty for paintress today.
type ComputedConfig struct{}
```

Add field to `ProjectConfig`:

```go
type ProjectConfig struct {
 // ... existing fields ...
 Computed ComputedConfig `yaml:"computed,omitempty"`
}
```

**Step 4: Run tests**

Run: `cd /Users/nino/tap/paintress && go test ./... -count=1 -timeout=300s`
Expected: ALL PASS

**Step 5: Commit**

```bash
git -C /Users/nino/tap/paintress add internal/domain/config.go internal/domain/config_test.go
git -C /Users/nino/tap/paintress commit -m "refactor: add empty ComputedConfig to ProjectConfig

[STRUCTURAL] Establishes UserConfig/ComputedConfig pattern for consistency.
ComputedConfig is empty and omitted from YAML output."
```

---

### Task 5: Fix config bypass in paintress doctor.go and issues.go

**Files:**

- Modify: `/Users/nino/tap/paintress/internal/cmd/doctor.go`
- Modify: `/Users/nino/tap/paintress/internal/cmd/issues.go`
- Create: `/Users/nino/tap/paintress/internal/cmd/config_helpers.go` (shared helper)
- Modify: `/Users/nino/tap/paintress/internal/cmd/doctor_test.go`

**Step 1: Write failing test for doctor using config-loaded claude_cmd**

In `/Users/nino/tap/paintress/internal/cmd/doctor_test.go`, add:

```go
func TestDoctor_UsesConfigClaudeCmd(t *testing.T) {
 // given — verify doctor command reads claude_cmd from config, not platform.DefaultClaudeCmd
 // This is a structural verification: the doctor command should call loadClaudeCmd()
 // which loads from ProjectConfig. We verify the helper exists and works.
 dir := t.TempDir()
 cmd := loadClaudeCmd(dir)
 // default should be "claude" (from DefaultProjectConfig)
 if cmd != "claude" {
  t.Errorf("loadClaudeCmd default = %q, want 'claude'", cmd)
 }
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/nino/tap/paintress && go test ./internal/cmd/ -run TestDoctor_UsesConfigClaudeCmd -v -count=1`
Expected: FAIL — `loadClaudeCmd` does not exist

**Step 3: Create config helper and fix doctor.go / issues.go**

Create `/Users/nino/tap/paintress/internal/cmd/config_helpers.go`:

```go
package cmd

import (
 "github.com/hironow/paintress/internal/session"
)

// loadClaudeCmd returns the claude_cmd from project config, falling back to default.
func loadClaudeCmd(repoPath string) string {
 cfg, err := session.LoadProjectConfig(repoPath)
 if err != nil {
  return "claude"
 }
 if cfg.ClaudeCmd != "" {
  return cfg.ClaudeCmd
 }
 return "claude"
}
```

Update `/Users/nino/tap/paintress/internal/cmd/doctor.go`:

- Replace `claudeCmd := platform.DefaultClaudeCmd` with `claudeCmd := loadClaudeCmd(continent)`
- Remove `platform` import if no longer used

Update `/Users/nino/tap/paintress/internal/cmd/issues.go`:

- Replace `platform.DefaultClaudeCmd` with `loadClaudeCmd(absPath)`

**Step 4: Run all tests**

Run: `cd /Users/nino/tap/paintress && go test ./... -count=1 -timeout=300s`
Expected: ALL PASS

**Step 5: Run lint**

Run: `cd /Users/nino/tap/paintress && just lint`
Expected: CLEAN

**Step 6: Commit**

```bash
git -C /Users/nino/tap/paintress add internal/cmd/config_helpers.go internal/cmd/doctor.go internal/cmd/issues.go internal/cmd/doctor_test.go
git -C /Users/nino/tap/paintress commit -m "fix: doctor and issues commands load claude_cmd from config

Replace hardcoded platform.DefaultClaudeCmd with config-loaded value.
Adds loadClaudeCmd() helper that falls back to default when config absent."
```

---

## Phase 3: amadeus — UserConfig / ComputedConfig + claude_cmd

amadeus has no computed fields but needs a `claude_cmd` field added to config and 4 hardcoded "claude" literals fixed.

### Task 6: Add ComputedConfig and claude_cmd to amadeus Config

**Files:**

- Modify: `/Users/nino/tap/amadeus/internal/domain/config.go`
- Modify: `/Users/nino/tap/amadeus/internal/domain/config_test.go`

**Step 1: Write failing tests**

In `/Users/nino/tap/amadeus/internal/domain/config_test.go`:

```go
func TestConfig_ComputedConfig_EmptyByDefault(t *testing.T) {
 cfg := DefaultConfig()
 if cfg.Computed != (ComputedConfig{}) {
  t.Errorf("ComputedConfig should be zero-value, got %+v", cfg.Computed)
 }
}

func TestDefaultConfig_ClaudeCmd(t *testing.T) {
 cfg := DefaultConfig()
 if cfg.ClaudeCmd != "claude" {
  t.Errorf("ClaudeCmd = %q, want 'claude'", cfg.ClaudeCmd)
 }
}

func TestValidateConfig_ClaudeCmdEmpty_IsValid(t *testing.T) {
 // empty claude_cmd is valid (uses default at runtime)
 cfg := DefaultConfig()
 cfg.ClaudeCmd = ""
 errs := ValidateConfig(cfg)
 for _, e := range errs {
  if strings.Contains(e, "claude_cmd") {
   t.Errorf("empty claude_cmd should be valid, got error: %s", e)
  }
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/nino/tap/amadeus && go test ./internal/domain/ -run 'TestConfig_ComputedConfig|TestDefaultConfig_ClaudeCmd|TestValidateConfig_ClaudeCmdEmpty' -v -count=1`
Expected: FAIL — `ComputedConfig` and `ClaudeCmd` field don't exist

**Step 3: Add types and field**

In `/Users/nino/tap/amadeus/internal/domain/config.go`:

```go
// ComputedConfig holds system-written fields. Empty for amadeus today.
type ComputedConfig struct{}

type Config struct {
 Lang            string            `yaml:"lang"`
 ClaudeCmd       string            `yaml:"claude_cmd,omitempty"`
 Weights         Weights           `yaml:"weights"`
 Thresholds      Thresholds        `yaml:"thresholds"`
 PerAxisOverride PerAxisOverride   `yaml:"per_axis_override"`
 FullCheck       FullCheckConfig   `yaml:"full_check"`
 Convergence     ConvergenceConfig `yaml:"convergence"`
 Computed        ComputedConfig    `yaml:"computed,omitempty"`
}
```

Update `DefaultConfig()` to set `ClaudeCmd: "claude"`.

**Step 4: Run tests**

Run: `cd /Users/nino/tap/amadeus && go test ./... -count=1 -timeout=300s`
Expected: ALL PASS

**Step 5: Commit**

```bash
git -C /Users/nino/tap/amadeus add internal/domain/config.go internal/domain/config_test.go
git -C /Users/nino/tap/amadeus commit -m "refactor: add ComputedConfig and claude_cmd field to Config

[STRUCTURAL] Establishes UserConfig/ComputedConfig pattern.
Adds claude_cmd to Config with default 'claude'."
```

---

### Task 7: Add claude_cmd to amadeus config set + fix hardcoded "claude"

**Files:**

- Modify: `/Users/nino/tap/amadeus/internal/cmd/config_cmd.go`
- Modify: `/Users/nino/tap/amadeus/internal/cmd/config_test.go`
- Modify: `/Users/nino/tap/amadeus/internal/cmd/check.go`
- Modify: `/Users/nino/tap/amadeus/internal/cmd/run.go`
- Modify: `/Users/nino/tap/amadeus/internal/session/claude.go`
- Modify: `/Users/nino/tap/amadeus/internal/session/review.go`

**Step 1: Write failing test for config set claude_cmd**

In `/Users/nino/tap/amadeus/internal/cmd/config_test.go`:

```go
func TestConfigSet_ClaudeCmd(t *testing.T) {
 cfg := domain.DefaultConfig()
 err := setAmadeusConfigField(&cfg, "claude_cmd", "custom-claude")
 if err != nil {
  t.Fatalf("setAmadeusConfigField: %v", err)
 }
 if cfg.ClaudeCmd != "custom-claude" {
  t.Errorf("ClaudeCmd = %q, want 'custom-claude'", cfg.ClaudeCmd)
 }
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/nino/tap/amadeus && go test ./internal/cmd/ -run TestConfigSet_ClaudeCmd -v -count=1`
Expected: FAIL — unknown config key "claude_cmd"

**Step 3: Add claude_cmd case to setAmadeusConfigField**

In `/Users/nino/tap/amadeus/internal/cmd/config_cmd.go`, add:

```go
case "claude_cmd":
 cfg.ClaudeCmd = value
```

**Step 4: Fix hardcoded "claude" in check.go, run.go, review.go, claude.go**

For each file, replace the hardcoded `"claude"` with config-loaded value. The exact approach depends on how config is passed in each command:

- **check.go:45**: Load config, use `cfg.ClaudeCmd` (with fallback to "claude" if empty)
- **run.go:47**: Same pattern
- **session/claude.go:28**: Accept claudeCmd parameter instead of hardcoding
- **session/review.go:148**: Already has fallback pattern, just change literal to `domain.DefaultConfig().ClaudeCmd`

**Step 5: Run all tests + lint**

Run: `cd /Users/nino/tap/amadeus && go test ./... -count=1 -timeout=300s && just lint`
Expected: ALL PASS, CLEAN

**Step 6: Commit**

```bash
git -C /Users/nino/tap/amadeus add internal/cmd/config_cmd.go internal/cmd/config_test.go internal/cmd/check.go internal/cmd/run.go internal/session/claude.go internal/session/review.go
git -C /Users/nino/tap/amadeus commit -m "fix: replace hardcoded 'claude' with config-loaded claude_cmd

Add claude_cmd to config set. Commands now load claude_cmd from
config.yaml with fallback to default when absent."
```

---

## Phase 4: Semgrep rules (all 3 tools)

### Task 8: Add semgrep rule — config-no-hardcoded-claude-cmd

**Files:**

- Modify: `/Users/nino/tap/paintress/.semgrep/layers.yaml`
- Modify: `/Users/nino/tap/amadeus/.semgrep/shared-adr.yaml` (or create `config.yaml`)

**Step 1: Write semgrep rule for paintress**

Add to `/Users/nino/tap/paintress/.semgrep/layers.yaml`:

```yaml
  - id: config-no-hardcoded-claude-cmd
    severity: ERROR
    message: >-
      Do not use platform.DefaultClaudeCmd outside cobra flag definitions.
      Load claude_cmd from config via loadClaudeCmd() or session.LoadProjectConfig().
    languages: [go]
    patterns:
      - pattern: platform.DefaultClaudeCmd
      - pattern-not-inside: |
          cmd.Flags().String("claude-cmd", platform.DefaultClaudeCmd, ...)
    paths:
      include:
        - internal/cmd/
        - internal/session/
        - internal/usecase/
      exclude:
        - '*_test.go'
        - 'internal/cmd/run.go'
```

**Step 2: Write semgrep rule for amadeus**

Add to amadeus semgrep config:

```yaml
  - id: config-no-hardcoded-claude-literal
    severity: ERROR
    message: >-
      Do not hardcode "claude" as exec argument. Use config.ClaudeCmd loaded from config.yaml.
    languages: [go]
    patterns:
      - pattern-either:
          - pattern: exec.CommandContext($CTX, "claude", ...)
          - pattern: exec.Command("claude", ...)
          - pattern: |
              append($BINS, "claude")
    paths:
      include:
        - internal/cmd/
        - internal/session/
      exclude:
        - '*_test.go'
```

**Step 3: Run semgrep to verify no violations**

Run: `cd /Users/nino/tap/paintress && just semgrep`
Run: `cd /Users/nino/tap/amadeus && just semgrep`
Expected: CLEAN (violations were already fixed in Tasks 5 and 7)

**Step 4: Commit**

```bash
git -C /Users/nino/tap/paintress add .semgrep/
git -C /Users/nino/tap/paintress commit -m "ci: add semgrep rule blocking hardcoded DefaultClaudeCmd"

git -C /Users/nino/tap/amadeus add .semgrep/
git -C /Users/nino/tap/amadeus commit -m "ci: add semgrep rule blocking hardcoded claude literal"
```

---

### Task 9: Add semgrep rule — config-no-computed-field-in-set (sightjack)

**Files:**

- Modify: `/Users/nino/tap/sightjack/.semgrep/layers.yaml`

**Step 1: Write semgrep rule**

```yaml
  - id: config-no-computed-field-in-set
    severity: ERROR
    message: >-
      Do not assign to ComputedConfig fields in setConfigField.
      Computed fields are read-only; use WriteEstimatedStrictness() for system writes.
    languages: [go]
    patterns:
      - pattern: cfg.Computed.$FIELD = $VALUE
      - pattern-inside: |
          func setConfigField(...) error {
            ...
          }
    paths:
      include:
        - internal/session/config.go
```

**Step 2: Run semgrep**

Run: `cd /Users/nino/tap/sightjack && just semgrep`
Expected: CLEAN

**Step 3: Commit**

```bash
git -C /Users/nino/tap/sightjack add .semgrep/layers.yaml
git -C /Users/nino/tap/sightjack commit -m "ci: add semgrep rule blocking computed field mutation in setConfigField"
```

---

### Task 10: Add semgrep rule — config-no-direct-computed-write (sightjack)

**Files:**

- Modify: `/Users/nino/tap/sightjack/.semgrep/layers.yaml`

**Step 1: Write semgrep rule**

```yaml
  - id: config-no-direct-computed-write
    severity: ERROR
    message: >-
      Do not write to ComputedConfig fields from cmd layer.
      Use session.WriteEstimatedStrictness() for system-level writes only.
    languages: [go]
    patterns:
      - pattern: $CFG.Computed.EstimatedStrictness = $VALUE
    paths:
      include:
        - internal/cmd/
      exclude:
        - '*_test.go'
```

**Step 2: Run semgrep**

Run: `cd /Users/nino/tap/sightjack && just semgrep`
Expected: CLEAN

**Step 3: Commit**

```bash
git -C /Users/nino/tap/sightjack add .semgrep/layers.yaml
git -C /Users/nino/tap/sightjack commit -m "ci: add semgrep rule blocking cmd-layer writes to ComputedConfig"
```

---

## Phase 5: Config round-trip integration tests

### Task 11: Config round-trip test for each tool

**Files:**

- Modify: `/Users/nino/tap/sightjack/internal/session/config_test.go`
- Modify: `/Users/nino/tap/paintress/internal/session/project_config_test.go`
- Modify: `/Users/nino/tap/amadeus/internal/cmd/config_test.go`

**Step 1: Write round-trip tests**

For each tool, write a test that:

1. Creates DefaultConfig()
2. Saves to YAML file
3. Loads from YAML file
4. Verifies all fields match

**sightjack** — in `config_test.go`:

```go
func TestConfig_SaveLoadRoundTrip_AllFields(t *testing.T) {
 // given
 dir := t.TempDir()
 cfgPath := filepath.Join(dir, "config.yaml")
 original := domain.DefaultConfig()

 // when — save
 data, err := yaml.Marshal(original)
 if err != nil {
  t.Fatalf("marshal: %v", err)
 }
 os.WriteFile(cfgPath, data, 0644)

 // when — load
 loaded, loadErr := LoadConfig(cfgPath)
 if loadErr != nil {
  t.Fatalf("LoadConfig: %v", loadErr)
 }

 // then — all user-config fields match
 if loaded.Lang != original.Lang {
  t.Errorf("Lang: got %q, want %q", loaded.Lang, original.Lang)
 }
 if loaded.Scan.ChunkSize != original.Scan.ChunkSize {
  t.Errorf("Scan.ChunkSize: got %d, want %d", loaded.Scan.ChunkSize, original.Scan.ChunkSize)
 }
 if loaded.Scan.MaxConcurrency != original.Scan.MaxConcurrency {
  t.Errorf("Scan.MaxConcurrency: got %d, want %d", loaded.Scan.MaxConcurrency, original.Scan.MaxConcurrency)
 }
 if loaded.Strictness.Default != original.Strictness.Default {
  t.Errorf("Strictness.Default: got %v, want %v", loaded.Strictness.Default, original.Strictness.Default)
 }
 if loaded.Retry.MaxAttempts != original.Retry.MaxAttempts {
  t.Errorf("Retry.MaxAttempts: got %d, want %d", loaded.Retry.MaxAttempts, original.Retry.MaxAttempts)
 }
 // ComputedConfig should be empty
 if loaded.Computed.EstimatedStrictness != nil {
  t.Errorf("Computed.EstimatedStrictness should be nil, got %v", loaded.Computed.EstimatedStrictness)
 }
}
```

**paintress** — in `project_config_test.go`:

```go
func TestProjectConfig_SaveLoadRoundTrip_AllFields(t *testing.T) {
 dir := t.TempDir()
 original := domain.DefaultProjectConfig()
 session.SaveProjectConfig(dir, &original)
 loaded, err := session.LoadProjectConfig(dir)
 if err != nil {
  t.Fatalf("LoadProjectConfig: %v", err)
 }
 if loaded.Model != original.Model {
  t.Errorf("Model: got %q, want %q", loaded.Model, original.Model)
 }
 if loaded.Workers != original.Workers {
  t.Errorf("Workers: got %d, want %d", loaded.Workers, original.Workers)
 }
 if loaded.ClaudeCmd != original.ClaudeCmd {
  t.Errorf("ClaudeCmd: got %q, want %q", loaded.ClaudeCmd, original.ClaudeCmd)
 }
 if loaded.Computed != (domain.ComputedConfig{}) {
  t.Errorf("Computed should be zero-value")
 }
}
```

**amadeus** — in `config_test.go`:

```go
func TestConfig_SaveLoadRoundTrip_AllFields(t *testing.T) {
 dir := t.TempDir()
 cfgPath := filepath.Join(dir, "config.yaml")
 original := domain.DefaultConfig()
 data, _ := yaml.Marshal(original)
 os.WriteFile(cfgPath, data, 0644)

 loaded, err := loadConfig(cfgPath)
 if err != nil {
  t.Fatalf("loadConfig: %v", err)
 }
 if loaded.Lang != original.Lang {
  t.Errorf("Lang: got %q, want %q", loaded.Lang, original.Lang)
 }
 if loaded.ClaudeCmd != original.ClaudeCmd {
  t.Errorf("ClaudeCmd: got %q, want %q", loaded.ClaudeCmd, original.ClaudeCmd)
 }
 if loaded.Computed != (domain.ComputedConfig{}) {
  t.Errorf("Computed should be zero-value")
 }
}
```

**Step 2: Run tests**

Run each tool's tests:

```bash
cd /Users/nino/tap/sightjack && go test ./internal/session/ -run TestConfig_SaveLoadRoundTrip -v -count=1
cd /Users/nino/tap/paintress && go test ./internal/session/ -run TestProjectConfig_SaveLoadRoundTrip -v -count=1
cd /Users/nino/tap/amadeus && go test ./internal/cmd/ -run TestConfig_SaveLoadRoundTrip -v -count=1
```

Expected: ALL PASS

**Step 3: Commit per tool**

```bash
git -C /Users/nino/tap/sightjack add internal/session/config_test.go
git -C /Users/nino/tap/sightjack commit -m "test: add config round-trip test verifying all fields"

git -C /Users/nino/tap/paintress add internal/session/project_config_test.go
git -C /Users/nino/tap/paintress commit -m "test: add config round-trip test verifying all fields"

git -C /Users/nino/tap/amadeus add internal/cmd/config_test.go
git -C /Users/nino/tap/amadeus commit -m "test: add config round-trip test verifying all fields"
```

---

## Phase 6: Final verification

### Task 12: Full check on all 3 tools

**Step 1: Run just check on all tools**

```bash
cd /Users/nino/tap/sightjack && just check
cd /Users/nino/tap/paintress && just check
cd /Users/nino/tap/amadeus && just check
```

Expected: ALL CLEAN — tests pass, lint passes, semgrep passes.

**Step 2: Verify no regressions with scenario tests (if available)**

```bash
cd /Users/nino/tap/sightjack && just test-scenario 2>/dev/null || echo "scenario tests skipped"
cd /Users/nino/tap/paintress && just test-scenario 2>/dev/null || echo "scenario tests skipped"
cd /Users/nino/tap/amadeus && just test-scenario 2>/dev/null || echo "scenario tests skipped"
```

---

## Task Dependency Graph

```
Task 1 (sightjack: reject computed key)
    |
    v
Task 2 (sightjack: UserConfig/ComputedConfig types)
    |
    v
Task 3 (sightjack: WriteEstimatedStrictness test)
    |
    +---> Task 9 (sightjack: semgrep computed-in-set)
    |         |
    |         v
    |     Task 10 (sightjack: semgrep direct-computed-write)
    |
Task 4 (paintress: ComputedConfig)
    |
    v
Task 5 (paintress: fix config bypass)
    |
    +---> Task 8a (paintress: semgrep hardcoded-claude-cmd)
    |
Task 6 (amadeus: ComputedConfig + claude_cmd)
    |
    v
Task 7 (amadeus: fix hardcoded "claude")
    |
    +---> Task 8b (amadeus: semgrep hardcoded-claude)
    |
    v
Task 11 (all: round-trip tests)
    |
    v
Task 12 (all: final verification)
```

Tasks 1-3 (sightjack), Tasks 4-5 (paintress), Tasks 6-7 (amadeus) can run in parallel across tools.
Semgrep rules (Tasks 8-10) depend on bypass fixes being complete.
Round-trip tests (Task 11) depend on all structural changes.
