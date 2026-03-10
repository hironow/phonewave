# Insight Ledger Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a unified `insights/` directory to each tool's state dir that stores semantic value (what/why/how/when/who/constraints) in git-tracked Markdown files, with atomic writes, D-Mail context attachment, and prerequisite ADRs.

**Architecture:** Shared `internal/domain/insight.go` types in each tool define the Insight Entry struct and format/parse logic. Each tool's session layer writes insights atomically (temp+rename+flock). D-Mail structs gain an optional `context` field for cross-tool insight summaries. Two new shared ADRs (S0030, S0031) formalize the persistence boundary change and D-Mail extension.

**Tech Stack:** Go 1.26, YAML frontmatter + Markdown, `syscall.Flock` (Unix) / `LockFileEx` (Windows), existing Cobra CLI + event sourcing

**Design doc:** `docs/plans/2026-03-10-insight-ledger-design.md`

---

## Task Dependencies

```
Task 1 (ADRs)          -- independent, prerequisite for all others
Task 2 (domain types)  -- depends on Task 1 (needs schema version reference)
Task 3 (atomic writer) -- depends on Task 2 (needs InsightEntry type)
Task 4 (gitignore)     -- independent
Task 5 (init dirs)     -- depends on Task 4 (gitignore must be ready)
Task 6 (D-Mail context)-- depends on Task 2 (needs InsightContext type)
Task 7 (paintress)     -- depends on Tasks 3, 5, 6
Task 8 (sightjack)     -- depends on Tasks 3, 5, 6
Task 9 (amadeus)       -- depends on Tasks 3, 5, 6
Task 10 (phonewave)    -- depends on Tasks 3, 5, 6
```

Tasks 1, 4 are independent.
Tasks 2, 3, 5, 6 form a sequential chain.
Tasks 7-10 are independent of each other (one per tool).

---

### Task 1: Write Shared ADRs S0030 and S0031

**Files:**
- Create: `phonewave/docs/shared-adr/S0030-insight-data-persistence.md`
- Create: `phonewave/docs/shared-adr/S0031-dmail-context-extension.md`
- Modify: `phonewave/docs/shared-adr/S0017-data-persistence-boundaries.md` (status line only)
- Modify: `phonewave/docs/shared-adr/S0005-dmail-schema-v1.md` (status line only)

**Step 1: Create S0030**

```markdown
# S0030. Insight Data Persistence (Supersedes S0017)

**Date:** 2026-03-10
**Status:** Accepted

## Context

S0017 established that all state directory contents are gitignored. However, tools
accumulate semantic value through feedback loops (Lumina patterns, Shibito warnings,
Convergence alerts, Divergence trends) that should persist across sessions and be
shared via git.

This value is environment-independent, contains no absolute paths, and is pure
semantic content — distinct from runtime state (`.run/`), events (`events/`), and
transient D-Mails (`inbox/outbox/archive/`).

## Decision

Add a new persistence category **"insight data"** to the data persistence boundaries:

| Category | Location | Git-tracked | Content |
|----------|----------|-------------|---------|
| State/cache | `.run/` | No | SQLite, runtime logs |
| Events | `events/` | No | JSONL event store |
| D-Mail | `inbox/outbox/archive/` | No | Inter-tool messages |
| Config | `config.yaml` | Varies | Tool settings |
| **Insight data** | **`insights/`** | **Yes** | **Semantic knowledge (what/why/how)** |

### Gitignore Strategy

State dir contents are gitignored individually (not the parent dir), allowing
`insights/` to remain tracked:

` ` `gitignore
# Tool runtime state — individual ignores (insights/ is git-tracked)
.expedition/.run/
.expedition/events/
.expedition/inbox/
.expedition/outbox/
.expedition/archive/
.expedition/journal/
.expedition/skills/
.expedition/config.yaml
.expedition/.otel.env
.expedition/.gitignore
` ` `

### Insight File Rules

- Files use `insight-schema-version: "1"` YAML frontmatter
- Content is environment-independent (no absolute paths, no machine-specific data)
- Atomic writes via temp-file + rename; concurrent access via flock
- Lock file stored in `.run/insights.lock` (gitignored)

## Consequences

### Positive
- Accumulated knowledge persists across sessions and developers via git
- Clear separation: insight data is semantic, not runtime state

### Negative
- Gitignore becomes individual-entry pattern instead of parent-dir pattern
- New state subdirectories require gitignore updates

### Neutral
- Supersedes S0017 — all S0017 rules remain except for the new insights category
- `*.local.*` pattern for env-specific data is unchanged
```

**Step 2: Create S0031**

```markdown
# S0031. D-Mail Context Extension (Amends S0005)

**Date:** 2026-03-10
**Status:** Accepted

## Context

S0005 defined the D-Mail v1 envelope with fixed fields. Tools need to attach
contextual insight summaries to D-Mails for cross-tool knowledge propagation
without introducing side-channel file reads.

## Decision

Add an optional `context` field to the D-Mail v1 envelope:

` ` `yaml
---
dmail-schema-version: "1"
name: spec-001
kind: specification
description: Implementation specification
context:
  insights:
    - source: ".expedition/insights/lumina.md"
      summary: "auth module CI不安定、stacked PR注意"
---
` ` `

### Rules

1. `context` is optional — omission is valid
2. `context.insights` is a list of `{source, summary}` pairs
3. Schema version remains "1" (additive, backward-compatible)
4. Receivers MUST NOT reject unknown context fields (S0019 Postel's Law)
5. phonewave relays context without interpretation

### Kind Validation Update

Add to the known kind set (already implemented, not yet in S0005):
- `implementation-feedback`
- `design-feedback`

## Consequences

### Positive
- Cross-tool insight propagation via D-Mail contract (no side-channel reads)
- Backward-compatible — existing parsers ignore unknown fields

### Negative
- All tools' D-Mail parsers need `context` field in struct

### Neutral
- Amends S0005 (adds field, does not change existing fields)
```

**Step 3: Update S0017 and S0005 status lines**

In S0017: Change `**Status:** Accepted` to `**Status:** Superseded by S0030`
In S0005: Change `**Status:** Accepted` to `**Status:** Amended by S0031`

**Step 4: Copy ADRs to other tools**

```bash
for tool in sightjack paintress amadeus; do
  cp phonewave/docs/shared-adr/S0030-insight-data-persistence.md $tool/docs/shared-adr/
  cp phonewave/docs/shared-adr/S0031-dmail-context-extension.md $tool/docs/shared-adr/
  cp phonewave/docs/shared-adr/S0017-data-persistence-boundaries.md $tool/docs/shared-adr/
  cp phonewave/docs/shared-adr/S0005-dmail-schema-v1.md $tool/docs/shared-adr/
done
```

**Step 5: Commit**

```bash
cd /Users/nino/tap/phonewave && git add docs/shared-adr/S0030* docs/shared-adr/S0031* docs/shared-adr/S0017* docs/shared-adr/S0005* && git commit -m "docs: add S0030 (insight data persistence) and S0031 (D-Mail context extension)"
# Repeat for each tool
```

---

### Task 2: Shared Domain Types — InsightEntry, InsightFile, InsightContext

This task creates the domain types that all tools share. The types are identical across tools (copy-paste, not a shared Go module — each tool is its own repo).

**Files:**
- Create: `phonewave/internal/domain/insight.go`
- Create: `phonewave/internal/domain/insight_test.go`
- Then copy to: `sightjack/`, `paintress/`, `amadeus/` (same paths)

**Step 1: Write failing tests**

File: `phonewave/internal/domain/insight_test.go`

```go
package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestInsightEntry_Format(t *testing.T) {
	entry := domain.InsightEntry{
		Title: "auth module CI flaky",
		What:  "3 consecutive CI failures on auth module changes",
		Why:   "OAuth token refresh times out in GitHub Actions network",
		How:   "Extend OAuth timeout to 30s in CI environment",
		When:  "CI environment with auth module changes",
		Who:   "paintress expedition #28, #30, #31",
		Constraints: "May self-resolve with OAuth provider changes",
		Extra: map[string]string{
			"failure-type":  "ci-red",
			"gradient-level": "0",
		},
	}

	formatted := entry.Format()

	if !strings.Contains(formatted, "## Insight: auth module CI flaky") {
		t.Errorf("missing title heading, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "- **what**: 3 consecutive") {
		t.Errorf("missing what field, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "- **failure-type**: ci-red") {
		t.Errorf("missing extra field, got:\n%s", formatted)
	}
}

func TestInsightFile_Marshal(t *testing.T) {
	now := time.Date(2026, 3, 10, 15, 30, 0, 0, time.FixedZone("JST", 9*3600))
	file := domain.InsightFile{
		SchemaVersion: "1",
		Kind:          "lumina",
		Tool:          "paintress",
		UpdatedAt:     now,
		Entries: []domain.InsightEntry{
			{
				Title: "test insight",
				What:  "observed X",
				Why:   "because Y",
				How:   "do Z",
				When:  "always",
				Who:   "test",
				Constraints: "none",
			},
		},
	}

	data, err := file.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		t.Errorf("should start with YAML frontmatter delimiter, got:\n%s", text)
	}
	if !strings.Contains(text, "insight-schema-version: \"1\"") {
		t.Errorf("missing schema version, got:\n%s", text)
	}
	if !strings.Contains(text, "entries: 1") {
		t.Errorf("missing entry count, got:\n%s", text)
	}
	if !strings.Contains(text, "## Insight: test insight") {
		t.Errorf("missing insight entry, got:\n%s", text)
	}
}

func TestInsightFile_Unmarshal(t *testing.T) {
	raw := `---
insight-schema-version: "1"
kind: lumina
tool: paintress
updated_at: "2026-03-10T15:30:00+09:00"
entries: 1
---

## Insight: test insight

- **what**: observed X
- **why**: because Y
- **how**: do Z
- **when**: always
- **who**: test
- **constraints**: none
- **failure-type**: ci-red
`

	file, err := domain.UnmarshalInsightFile([]byte(raw))
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if file.Kind != "lumina" {
		t.Errorf("expected kind lumina, got %s", file.Kind)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
	if file.Entries[0].Title != "test insight" {
		t.Errorf("expected title 'test insight', got %q", file.Entries[0].Title)
	}
	if file.Entries[0].Extra["failure-type"] != "ci-red" {
		t.Errorf("expected extra failure-type ci-red, got %q", file.Entries[0].Extra["failure-type"])
	}
}

func TestInsightContext_Format(t *testing.T) {
	ctx := domain.InsightContext{
		Insights: []domain.InsightSummary{
			{Source: ".expedition/insights/lumina.md", Summary: "auth CI flaky"},
		},
	}

	if ctx.Insights[0].Source != ".expedition/insights/lumina.md" {
		t.Errorf("unexpected source: %s", ctx.Insights[0].Source)
	}
}
```

**Step 2: Run tests — expect FAIL**

```bash
cd /Users/nino/tap/phonewave && go test ./internal/domain/ -run TestInsight -v
```

Expected: compilation error (types not defined)

**Step 3: Implement domain types**

File: `phonewave/internal/domain/insight.go`

```go
package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const InsightSchemaVersion = "1"

// InsightEntry represents a single semantic insight with 6 required axes + optional extras.
type InsightEntry struct {
	Title       string
	What        string
	Why         string
	How         string
	When        string
	Who         string
	Constraints string
	Extra       map[string]string // tool-specific optional fields
}

// InsightFile is the on-disk representation of an insight ledger file.
type InsightFile struct {
	SchemaVersion string         `yaml:"insight-schema-version"`
	Kind          string         `yaml:"kind"`
	Tool          string         `yaml:"tool"`
	UpdatedAt     time.Time      `yaml:"updated_at"`
	Entries       []InsightEntry `yaml:"-"` // parsed from Markdown body
}

// insightFrontmatter is the YAML-only portion for marshal/unmarshal.
type insightFrontmatter struct {
	SchemaVersion string `yaml:"insight-schema-version"`
	Kind          string `yaml:"kind"`
	Tool          string `yaml:"tool"`
	UpdatedAt     string `yaml:"updated_at"`
	EntryCount    int    `yaml:"entries"`
}

// InsightContext is the optional context field added to D-Mail envelopes.
type InsightContext struct {
	Insights []InsightSummary `yaml:"insights,omitempty" json:"insights,omitempty"`
}

// InsightSummary is a single insight reference within a D-Mail context.
type InsightSummary struct {
	Source  string `yaml:"source" json:"source"`
	Summary string `yaml:"summary" json:"summary"`
}

// Format renders a single InsightEntry as Markdown.
func (e InsightEntry) Format() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Insight: %s\n\n", e.Title)
	fmt.Fprintf(&sb, "- **what**: %s\n", e.What)
	fmt.Fprintf(&sb, "- **why**: %s\n", e.Why)
	fmt.Fprintf(&sb, "- **how**: %s\n", e.How)
	fmt.Fprintf(&sb, "- **when**: %s\n", e.When)
	fmt.Fprintf(&sb, "- **who**: %s\n", e.Who)
	fmt.Fprintf(&sb, "- **constraints**: %s\n", e.Constraints)

	// Extra fields sorted for deterministic output
	if len(e.Extra) > 0 {
		keys := make([]string, 0, len(e.Extra))
		for k := range e.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "- **%s**: %s\n", k, e.Extra[k])
		}
	}
	return sb.String()
}

// Marshal renders the full InsightFile as YAML frontmatter + Markdown body.
func (f InsightFile) Marshal() ([]byte, error) {
	fm := insightFrontmatter{
		SchemaVersion: f.SchemaVersion,
		Kind:          f.Kind,
		Tool:          f.Tool,
		UpdatedAt:     f.UpdatedAt.Format(time.RFC3339),
		EntryCount:    len(f.Entries),
	}
	header, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshal insight frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(header)
	sb.WriteString("---\n")

	for i, entry := range f.Entries {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(entry.Format())
	}
	return []byte(sb.String()), nil
}

var insightHeadingRe = regexp.MustCompile(`(?m)^## Insight: (.+)$`)
var insightFieldRe = regexp.MustCompile(`(?m)^- \*\*([^*]+)\*\*: (.+)$`)

// UnmarshalInsightFile parses a YAML frontmatter + Markdown insight file.
func UnmarshalInsightFile(data []byte) (*InsightFile, error) {
	text := string(data)
	parts := strings.SplitN(text, "---\n", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid insight file: missing YAML frontmatter delimiters")
	}

	var fm insightFrontmatter
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return nil, fmt.Errorf("unmarshal insight frontmatter: %w", err)
	}

	updatedAt, err := time.Parse(time.RFC3339, fm.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	file := &InsightFile{
		SchemaVersion: fm.SchemaVersion,
		Kind:          fm.Kind,
		Tool:          fm.Tool,
		UpdatedAt:     updatedAt,
	}

	// Parse Markdown body into entries
	body := parts[2]
	headings := insightHeadingRe.FindAllStringSubmatchIndex(body, -1)

	for i, loc := range headings {
		title := body[loc[2]:loc[3]]
		// Determine entry body range
		start := loc[1]
		end := len(body)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		entryBody := body[start:end]

		entry := InsightEntry{Title: title, Extra: make(map[string]string)}
		fields := insightFieldRe.FindAllStringSubmatch(entryBody, -1)
		for _, f := range fields {
			key, value := f[1], f[2]
			switch key {
			case "what":
				entry.What = value
			case "why":
				entry.Why = value
			case "how":
				entry.How = value
			case "when":
				entry.When = value
			case "who":
				entry.Who = value
			case "constraints":
				entry.Constraints = value
			default:
				entry.Extra[key] = value
			}
		}
		file.Entries = append(file.Entries, entry)
	}

	return file, nil
}
```

**Step 4: Run tests — expect PASS**

```bash
cd /Users/nino/tap/phonewave && go test ./internal/domain/ -run TestInsight -v
```

**Step 5: Copy to other tools**

Copy `insight.go` and `insight_test.go` to each tool, updating the import path:

```bash
for tool in sightjack paintress amadeus; do
  cp phonewave/internal/domain/insight.go /Users/nino/tap/$tool/internal/domain/insight.go
  cp phonewave/internal/domain/insight_test.go /Users/nino/tap/$tool/internal/domain/insight_test.go
  # Fix import path
  sed -i '' "s|github.com/hironow/phonewave|github.com/hironow/$tool|g" /Users/nino/tap/$tool/internal/domain/insight.go /Users/nino/tap/$tool/internal/domain/insight_test.go
done
```

Run tests in each tool:
```bash
for tool in phonewave sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && go test ./internal/domain/ -run TestInsight -v
done
```

**Step 6: Commit in each tool**

```bash
for tool in phonewave sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && git add internal/domain/insight.go internal/domain/insight_test.go && git commit -m "feat: add InsightEntry and InsightFile domain types (insight-schema-version 1)"
done
```

---

### Task 3: Atomic Insight Writer (session layer)

**Files:**
- Create: `phonewave/internal/session/insight_writer.go`
- Create: `phonewave/internal/session/insight_writer_test.go`
- Then copy to all tools (with import path fix)

**Step 1: Write failing tests**

File: `phonewave/internal/session/insight_writer_test.go`

```go
package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestInsightWriter_WriteNew(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{
		Title: "test insight",
		What:  "observed X",
		Why:   "because Y",
		How:   "do Z",
		When:  "always",
		Who:   "test",
		Constraints: "none",
	}

	err := w.Append("test.md", "test-kind", "test-tool", entry)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(filepath.Join(insightsDir, "test.md"))
	if err != nil {
		t.Fatalf("read insight file: %v", err)
	}

	file, err := domain.UnmarshalInsightFile(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
	if file.Entries[0].Title != "test insight" {
		t.Errorf("expected title 'test insight', got %q", file.Entries[0].Title)
	}
	if file.Kind != "test-kind" {
		t.Errorf("expected kind 'test-kind', got %q", file.Kind)
	}
}

func TestInsightWriter_AppendExisting(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	e1 := domain.InsightEntry{Title: "first", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}
	e2 := domain.InsightEntry{Title: "second", What: "g", Why: "h", How: "i", When: "j", Who: "k", Constraints: "l"}

	if err := w.Append("multi.md", "lumina", "paintress", e1); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := w.Append("multi.md", "lumina", "paintress", e2); err != nil {
		t.Fatalf("second append: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(insightsDir, "multi.md"))
	file, _ := domain.UnmarshalInsightFile(data)

	if len(file.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(file.Entries))
	}
	if file.Entries[0].Title != "first" {
		t.Errorf("first entry title: %q", file.Entries[0].Title)
	}
	if file.Entries[1].Title != "second" {
		t.Errorf("second entry title: %q", file.Entries[1].Title)
	}
}

func TestInsightWriter_AtomicNoCorruption(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "atomic", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	_ = w.Append("atomic.md", "test", "test", entry)

	// No temp files should remain
	matches, _ := filepath.Glob(filepath.Join(insightsDir, ".*.tmp"))
	if len(matches) > 0 {
		t.Errorf("temp files should be cleaned up, found: %v", matches)
	}
}

func TestInsightWriter_IdempotentAppend(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	entry := domain.InsightEntry{Title: "dedup me", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	// Append twice with same title
	if err := w.Append("dedup.md", "test", "test", entry); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := w.Append("dedup.md", "test", "test", entry); err != nil {
		t.Fatalf("second append: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(insightsDir, "dedup.md"))
	file, _ := domain.UnmarshalInsightFile(data)

	if len(file.Entries) != 1 {
		t.Errorf("expected 1 entry (idempotent), got %d", len(file.Entries))
	}
}

func TestInsightWriter_PropagatesNonENOENT(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	// Create a corrupt file (invalid YAML frontmatter)
	os.WriteFile(filepath.Join(insightsDir, "corrupt.md"), []byte("not valid insight file"), 0o644)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "test", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}

	err := w.Append("corrupt.md", "test", "test", entry)
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestInsightWriter_ReadEntries(t *testing.T) {
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)
	entry := domain.InsightEntry{Title: "readable", What: "a", Why: "b", How: "c", When: "d", Who: "e", Constraints: "f"}
	_ = w.Append("read.md", "lumina", "paintress", entry)

	file, err := w.Read("read.md")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
}
```

**Step 2: Run tests — expect FAIL**

```bash
cd /Users/nino/tap/phonewave && go test ./internal/session/ -run TestInsightWriter -v
```

**Step 3: Implement InsightWriter**

File: `phonewave/internal/session/insight_writer.go`

```go
package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

// InsightWriter provides atomic read/write access to insight ledger files.
type InsightWriter struct {
	insightsDir string
	runDir      string
}

// NewInsightWriter creates an InsightWriter for the given directories.
func NewInsightWriter(insightsDir, runDir string) *InsightWriter {
	return &InsightWriter{insightsDir: insightsDir, runDir: runDir}
}

// Append adds a new InsightEntry to the named file, creating it if needed.
// Uses flock + atomic rename for concurrent safety.
func (w *InsightWriter) Append(filename, kind, tool string, entry domain.InsightEntry) error {
	path := filepath.Join(w.insightsDir, filename)

	unlock, err := w.lock()
	if err != nil {
		return fmt.Errorf("acquire insight lock: %w", err)
	}
	defer unlock()

	// Read existing — only create new on ENOENT, propagate other errors
	file, err := w.readLocked(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read existing insight file %s: %w", path, err)
		}
		file = &domain.InsightFile{
			SchemaVersion: domain.InsightSchemaVersion,
			Kind:          kind,
			Tool:          tool,
		}
	}

	// Idempotency: skip if entry with same title already exists
	for _, existing := range file.Entries {
		if existing.Title == entry.Title {
			return nil // already recorded
		}
	}

	file.Entries = append(file.Entries, entry)
	file.UpdatedAt = time.Now()

	data, err := file.Marshal()
	if err != nil {
		return fmt.Errorf("marshal insight file: %w", err)
	}

	// Atomic write: temp file + rename
	tmpPath := filepath.Join(w.insightsDir, "."+filename+".tmp")
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp insight file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // clean up on failure
		return fmt.Errorf("rename insight file: %w", err)
	}

	return nil
}

// Read parses an insight file without locking (safe due to atomic rename).
func (w *InsightWriter) Read(filename string) (*domain.InsightFile, error) {
	path := filepath.Join(w.insightsDir, filename)
	return w.readLocked(path)
}

func (w *InsightWriter) readLocked(path string) (*domain.InsightFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return domain.UnmarshalInsightFile(data)
}
```

**Step 4: Implement platform-specific flock**

File: `phonewave/internal/session/insight_lock_unix.go`

```go
//go:build !windows

package session

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func (w *InsightWriter) lock() (func(), error) {
	lockPath := filepath.Join(w.runDir, "insights.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("flock %s: %w", lockPath, err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
		f.Close()
	}, nil
}
```

File: `phonewave/internal/session/insight_lock_windows.go`

```go
//go:build windows

package session

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func (w *InsightWriter) lock() (func(), error) {
	lockPath := filepath.Join(w.runDir, "insights.lock")
	p, err := windows.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, fmt.Errorf("convert lock path: %w", err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	// LockFileEx with LOCKFILE_EXCLUSIVE_LOCK (blocking)
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol); err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("LockFileEx %s: %w", lockPath, err)
	}
	return func() {
		windows.UnlockFileEx(h, 0, 1, 0, ol) //nolint:errcheck
		windows.CloseHandle(h)
	}, nil
}

// Ensure unsafe import is used (windows.Overlapped needs it indirectly).
var _ = unsafe.Sizeof(0)
```

**Step 5: Run tests — expect PASS**

```bash
cd /Users/nino/tap/phonewave && go test ./internal/session/ -run TestInsightWriter -v
```

**Step 6: Copy to other tools, fix imports, run tests**

```bash
for tool in sightjack paintress amadeus; do
  for f in insight_writer.go insight_writer_test.go insight_lock_unix.go insight_lock_windows.go; do
    cp /Users/nino/tap/phonewave/internal/session/$f /Users/nino/tap/$tool/internal/session/$f
    sed -i '' "s|github.com/hironow/phonewave|github.com/hironow/$tool|g" /Users/nino/tap/$tool/internal/session/$f
  done
  cd /Users/nino/tap/$tool && go test ./internal/session/ -run TestInsightWriter -v
done
```

**Step 7: Commit in each tool**

```bash
for tool in phonewave sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && git add internal/session/insight_writer.go internal/session/insight_writer_test.go internal/session/insight_lock_unix.go internal/session/insight_lock_windows.go && git commit -m "feat: add atomic InsightWriter with flock"
done
```

---

### Task 4: Gitignore Refactoring (all 4 tools)

**Files:**
- Modify: `phonewave/.gitignore`
- Modify: `sightjack/.gitignore`
- Modify: `paintress/.gitignore`
- Modify: `amadeus/.gitignore`

**Step 1: Update phonewave .gitignore**

Replace:
```gitignore
# phonewave runtime state on target repository
.phonewave/
```

With:
```gitignore
# phonewave runtime state — ignore contents individually (insights/ is git-tracked per S0030)
.phonewave/.run/
.phonewave/events/
.phonewave/inbox/
.phonewave/outbox/
.phonewave/archive/
.phonewave/skills/
.phonewave/config.yaml
.phonewave/.otel.env
.phonewave/.gitignore
```

**Step 2: Update sightjack .gitignore**

Replace:
```gitignore
# sightjack runtime state on target repository
.siren/
```

With:
```gitignore
# sightjack runtime state — ignore contents individually (insights/ is git-tracked per S0030)
.siren/.run/
.siren/events/
.siren/inbox/
.siren/outbox/
.siren/archive/
.siren/skills/
.siren/config.yaml
.siren/.otel.env
.siren/.gitignore
```

**Step 3: Update paintress .gitignore**

Replace:
```gitignore
# paintress runtime state on target repository
.expedition/
```

With:
```gitignore
# paintress runtime state — ignore contents individually (insights/ is git-tracked per S0030)
.expedition/.run/
.expedition/events/
.expedition/inbox/
.expedition/outbox/
.expedition/archive/
.expedition/journal/
.expedition/skills/
.expedition/config.yaml
.expedition/.otel.env
.expedition/.gitignore
```

**Step 4: Update amadeus .gitignore**

Replace:
```gitignore
# amadeus runtime state on target repository
.gate/
```

With:
```gitignore
# amadeus runtime state — ignore contents individually (insights/ is git-tracked per S0030)
.gate/.run/
.gate/events/
.gate/inbox/
.gate/outbox/
.gate/archive/
.gate/skills/
.gate/config.yaml
.gate/.otel.env
.gate/.gitignore
```

**Step 5: Verify gitignore works**

```bash
for tool in phonewave sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && git status
done
```

Ensure no previously-tracked files suddenly appear.

**Step 6: Commit in each tool**

```bash
for tool in phonewave sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && git add .gitignore && git commit -m "refactor: decompose state dir gitignore for insights/ tracking (S0030)"
done
```

---

### Task 5: Add `insights/` to State Dir Initialization

**Files:**
- Modify: `phonewave/internal/session/state.go` (around line 25-31, `EnsureStateDir`)
- Modify: `sightjack/internal/session/init_adapter.go` (around line 20-45, `InitProject`/`EnsureMailDirs`)
- Modify: `paintress/internal/cmd/init.go` or `paintress/internal/session/` (wherever subdirs are created)
- Modify: `amadeus/internal/session/state.go` (around line 39-120, `InitGateDir`)

**Step 1: For each tool, add `insights/` to the list of mkdir calls**

Pattern — find the block that creates subdirectories and add `insights/`:

```go
// Existing pattern in each tool:
for _, sub := range []string{".run", "events", "inbox", "outbox", "archive", /* tool-specific... */} {
    if err := os.MkdirAll(filepath.Join(stateDir, sub), 0o755); err != nil {
        return err
    }
}
// Add "insights" to the slice
```

Each tool may have a slightly different pattern — read the exact code before modifying.

**Step 2: Run init-related tests**

```bash
for tool in phonewave sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && go test ./... -run "Init|State|EnsureState" -v
done
```

**Step 3: Commit in each tool**

```bash
for tool in phonewave sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && git add internal/ && git commit -m "feat: add insights/ directory to state dir initialization"
done
```

---

### Task 6: Add `context` Field to D-Mail Struct

**Files:**
- Modify: `paintress/internal/domain/dmail.go` (~line 100-112)
- Modify: `sightjack/internal/session/dmail.go` (~line 23-34)
- Modify: `amadeus/internal/domain/dmail.go` (~line 82-94)
- Corresponding test files for each

**Step 1: Add Context field to DMail struct in each tool**

In each tool's DMail struct, add:

```go
Context *InsightContext `yaml:"context,omitempty" json:"context,omitempty"`
```

Using the `InsightContext` type from Task 2.

**Step 2: Write test for context round-trip**

In each tool's D-Mail test file, add a test that creates a DMail with context, marshals to YAML frontmatter, and verifies it parses back:

```go
func TestDMail_ContextRoundTrip(t *testing.T) {
	dm := DMail{
		SchemaVersion: "1",
		Name:          "test-001",
		Kind:          "specification",
		Description:   "test",
		Context: &InsightContext{
			Insights: []InsightSummary{
				{Source: ".expedition/insights/lumina.md", Summary: "auth CI flaky"},
			},
		},
		Body: "# Test",
	}
	data := dm.Format() // or equivalent marshal function
	parsed, err := ParseDMail([]byte(data))
	// verify parsed.Context is not nil
	// verify parsed.Context.Insights[0].Summary == "auth CI flaky"
}
```

Adapt to each tool's actual marshal/parse API.

**Step 3: Run tests**

```bash
for tool in sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && go test ./... -run "TestDMail" -v
done
```

**Step 4: Commit in each tool**

```bash
for tool in sightjack paintress amadeus; do
  cd /Users/nino/tap/$tool && git add internal/ && git commit -m "feat: add optional context field to D-Mail struct (S0031)"
done
```

---

### Task 7: paintress — Lumina & Gommage Insight Generation

**Files:**
- Modify: `paintress/internal/session/lumina.go` (~line 99+, after pattern aggregation)
- Modify: `paintress/internal/domain/escalation.go` (Gommage D-Mail creation)
- Create or modify test files

**Step 1: After `ScanJournalsForLumina` aggregates patterns, write to `insights/lumina.md`**

At the end of `ScanJournalsForLumina`, call `InsightWriter.Append` for each discovered Lumina pattern with enriched what/why/how fields.

The `Insight` field from journal entries already contains semantic "why" text. Map it:
- **what**: Pattern text (existing `Lumina.Pattern`)
- **why**: Extracted from journal's `Insight` field
- **how**: Derived from source category (`failure-pattern` → defensive strategy, `success-pattern` → continue doing)
- **when**: Extracted from journal's `Mission` type and `Issue` context
- **who**: Journal expedition number and session
- **constraints**: Source threshold context ("appeared N times")
- **failure-type** (extra): From journal status
- **gradient-level** (extra): Current gradient gauge level if available

**Step 2: On Gommage trigger, write to `insights/gommage.md`**

In `NewEscalationDMail` or the policy handler that triggers Gommage, append an insight:
- **what**: N consecutive failures on expedition
- **why**: Template: "Consecutive failures indicate systematic issue"
- **how**: "Review failure reasons in recent journal entries. Consider manual intervention"
- **when**: "When consecutive failure count reaches threshold"
- **who**: "paintress Gommage policy, expedition #N"
- **constraints**: "Counter resets on next success"
- **failure-type** (extra): Extracted from latest journal entry
- **gradient-level** (extra): "0 (discharged by Gommage)"

**Step 3: Tests**

Write integration-style tests that verify insight files are created after journal scan and gommage triggering.

**Step 4: Commit**

```bash
cd /Users/nino/tap/paintress && git add internal/ && git commit -m "feat: generate lumina and gommage insights from expedition feedback"
```

---

### Task 8: sightjack — Shibito & Strictness Insight Generation

**Files:**
- Modify: `sightjack/internal/session/scanner.go` (after scan pass completion)
- Create or modify test files

**Step 1: After scan produces ShibitoWarnings, write to `insights/shibito.md`**

For each `ShibitoWarning` in the scan result:
- **what**: `Description` field (already semantic)
- **why**: "Issue {ClosedIssueID} was closed but pattern re-emerged in {CurrentIssueID}"
- **how**: "Review the original fix for {ClosedIssueID}. Consider structural prevention"
- **when**: "During scan, when closed issue patterns match current open issues"
- **who**: "sightjack scan pass 1 (session-{id})"
- **constraints**: "Risk level: {RiskLevel}"
- **closed-issue-ids** (extra): ClosedIssueID
- **cluster** (extra): Cluster name if available

**Step 2: After strictness resolution, write to `insights/strictness.md`**

For each cluster with `EstimatedStrictness` and `StrictnessReasoning`:
- **what**: "Cluster {Name} estimated strictness: {EstimatedStrictness}"
- **why**: `StrictnessReasoning` field (already semantic — Claude generates this)
- **how**: "Manual override available via config if estimated level is too high/low"
- **when**: "During scan pass 2, per-cluster deep analysis"
- **who**: "sightjack scan pass 2 (session-{id})"
- **constraints**: "Estimated — may differ from manually configured override"
- **cluster** (extra): Cluster name
- **completeness-delta** (extra): If previous completeness available

**Step 3: Tests and commit**

```bash
cd /Users/nino/tap/sightjack && git add internal/ && git commit -m "feat: generate shibito and strictness insights from scan results"
```

---

### Task 9: amadeus — Divergence & Convergence Insight Generation

**Files:**
- Modify: `amadeus/internal/usecase/emitter.go` or check pipeline
- Modify: `amadeus/internal/domain/convergence.go` (GenerateConvergenceDMails area)
- Create or modify test files

**Step 1: After DetermineSeverity, write to `insights/divergence.md`**

After each check produces a `DivergenceResult`:
- **what**: "Divergence score: {Value}, severity: {Severity}"
- **why**: Concatenation of `AxisScore.Details` for each axis with score above threshold
- **how**: "Focus remediation on highest-scoring axis: {axis with max score}"
- **when**: "Check on commits {range}"
- **who**: "amadeus check (session-{id})"
- **constraints**: "Overridden: {yes/no}. Scores are relative to configured weights"
- **axis-scores** (extra): `adr:{N} dod:{N} dep:{N} implicit:{N}`
- **commits** (extra): Commit range

**Step 2: On ConvergenceAlert with HIGH severity, write to `insights/convergence.md`**

For each HIGH convergence alert:
- **what**: "World line convergence on {Target}: {Count} D-Mails in {Window} days"
- **why**: "Multiple feedback signals targeting same area indicates structural issue"
- **how**: "Investigate {Target} for shared root cause across {DMails}"
- **when**: "When {Count} D-Mails target same area within {Window}-day window"
- **who**: "amadeus convergence detector (session-{id})"
- **constraints**: "Escalation threshold: {threshold}x multiplier for HIGH"
- **related-dmails** (extra): D-Mail names

**Step 3: Tests and commit**

```bash
cd /Users/nino/tap/amadeus && git add internal/ && git commit -m "feat: generate divergence and convergence insights from check results"
```

---

### Task 10: phonewave — Delivery Insight Generation

**Files:**
- Modify: `phonewave/internal/usecase/policy_handlers.go` (on `delivery.failed` event)
- Create or modify test files

**Step 1: On delivery failure events, accumulate and write to `insights/delivery.md`**

After handling a `delivery.failed` event (or periodically after a scan cycle):
- **what**: "Delivery failed for kind {Kind} from {SourceOutbox}: {ErrorMessage}"
- **why**: Error message categorization (permission denied, target not found, disk full, etc.)
- **how**: "Check target inbox directory permissions and disk space"
- **when**: "During delivery scan cycle"
- **who**: "phonewave courier daemon (session-{id})"
- **constraints**: "Retry count: {N}. Max retries configured via backoff"
- **route** (extra): `{Kind}: {SourceOutbox} -> {TargetInbox}`
- **retry-count** (extra): Current retry count

**Step 2: Tests and commit**

```bash
cd /Users/nino/tap/phonewave && git add internal/ && git commit -m "feat: generate delivery pattern insights from courier events"
```

---

## Execution Order Summary

```
Phase 1 (Foundation, parallel):
  Task 1 — ADRs (S0030, S0031)
  Task 4 — Gitignore refactoring

Phase 2 (Core types, sequential):
  Task 2 — Domain types (InsightEntry, InsightFile, InsightContext)
  Task 3 — Atomic InsightWriter
  Task 5 — Init dir (depends on Task 4 gitignore)
  Task 6 — D-Mail context field

Phase 3 (Per-tool integration, parallel):
  Task 7  — paintress (lumina + gommage)
  Task 8  — sightjack (shibito + strictness)
  Task 9  — amadeus (divergence + convergence)
  Task 10 — phonewave (delivery)
```

Total: 10 tasks. Phase 1 and Phase 3 are parallelizable.
