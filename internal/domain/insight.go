package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// InsightSchemaVersion is the current schema version for insight files.
const InsightSchemaVersion = "1"

// InsightEntry represents a single semantic insight with 6 required axes + optional extras.
type InsightEntry struct { // nosemgrep: structure.multiple-exported-structs-go -- insight family (InsightEntry/InsightFile/InsightContext/InsightSummary) is cohesive insight-ledger type set [permanent]
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
type InsightFile struct { // nosemgrep: structure.multiple-exported-structs-go -- insight family; see InsightEntry [permanent]
	SchemaVersion string    `yaml:"insight-schema-version"`
	Kind          string    `yaml:"kind"`
	Tool          string    `yaml:"tool"`
	UpdatedAt     time.Time `yaml:"updated_at"`
	entries       []InsightEntry
}

// All returns the entries in the insight file.
func (f *InsightFile) All() []InsightEntry {
	return f.entries
}

// AddEntry appends entry if no existing entry has the same Title.
// Returns true if the entry was added, false if it was a duplicate.
func (f *InsightFile) AddEntry(entry InsightEntry) bool {
	for _, existing := range f.entries {
		if existing.Title == entry.Title {
			return false
		}
	}
	f.entries = append(f.entries, entry)
	return true
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
type InsightContext struct { // nosemgrep: structure.multiple-exported-structs-go,first-class-collection.raw-slice-field-domain-go -- insight family; see InsightEntry [permanent]
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
		EntryCount:    len(f.entries),
	}
	header, err := yaml.Marshal(fm)
	if err != nil {
		return nil, fmt.Errorf("marshal insight frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(header)
	sb.WriteString("---\n")

	for i, entry := range f.entries {
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
		file.entries = append(file.entries, entry)
	}

	return file, nil
}
