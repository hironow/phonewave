package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/tests/contract"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const goldenDir = "testdata/golden"

// goldenFiles returns the list of golden file names (without path).
func goldenFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		t.Fatal("no golden files found")
	}
	return files
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(goldenDir, name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return data
}

// ──────────────────────────────────────────────
// Group 1: Parse Compatibility
// All golden files must parse successfully with the Postel-liberal parser.
// ──────────────────────────────────────────────

func TestGroup1_ParseCompatibility(t *testing.T) {
	for _, name := range goldenFiles(t) {
		t.Run(name, func(t *testing.T) {
			data := readGolden(t, name)
			dm, err := contract.Parse(data)
			if err != nil {
				t.Fatalf("Parse(%s) error: %v", name, err)
			}
			// Required fields must be present
			if dm.Name == "" {
				t.Error("parsed name is empty")
			}
			if dm.Kind == "" {
				t.Error("parsed kind is empty")
			}
			if dm.Description == "" {
				t.Error("parsed description is empty")
			}
			if dm.SchemaVersion == "" {
				t.Error("parsed dmail-schema-version is empty")
			}
		})
	}
}

// ──────────────────────────────────────────────
// Group 2: Required Fields
// Tool-produced golden files must have all schema v1 required fields.
// ──────────────────────────────────────────────

func TestGroup2_RequiredFields(t *testing.T) {
	toolFiles := []string{
		"sightjack-spec.md",
		"sightjack-report.md",
		"sightjack-feedback.md",
		"paintress-report.md",
		"amadeus-feedback-high.md",
		"amadeus-convergence.md",
	}
	for _, name := range toolFiles {
		t.Run(name, func(t *testing.T) {
			data := readGolden(t, name)
			dm, err := contract.Parse(data)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if dm.SchemaVersion != "1" {
				t.Errorf("schema version = %q, want %q", dm.SchemaVersion, "1")
			}
			validKinds := map[string]bool{
				"specification": true, "report": true,
				"design-feedback": true, "implementation-feedback": true, "convergence": true,
			}
			if !validKinds[dm.Kind] {
				t.Errorf("kind = %q, not a valid schema v1 kind", dm.Kind)
			}
		})
	}
}

// ──────────────────────────────────────────────
// Group 3: Idempotency Key
// SHA256(name + \x00 + kind + \x00 + description + \x00 + body) must be
// consistent across parse-compute cycles.
// ──────────────────────────────────────────────

func TestGroup3_IdempotencyKey(t *testing.T) {
	for _, name := range goldenFiles(t) {
		t.Run(name, func(t *testing.T) {
			data := readGolden(t, name)
			dm, err := contract.Parse(data)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			key1 := contract.IdempotencyKey(dm)
			key2 := contract.IdempotencyKey(dm)
			if key1 != key2 {
				t.Errorf("idempotency key not stable: %s != %s", key1, key2)
			}
			if len(key1) != 64 { // SHA256 hex = 64 chars
				t.Errorf("idempotency key length = %d, want 64", len(key1))
			}
		})
	}
}

// TestGroup3_IdempotencyKeyDiffers verifies that different D-Mails produce
// different idempotency keys.
func TestGroup3_IdempotencyKeyDiffers(t *testing.T) {
	files := goldenFiles(t)
	if len(files) < 2 {
		t.Skip("need at least 2 golden files")
	}
	keys := make(map[string]string)
	for _, name := range files {
		data := readGolden(t, name)
		dm, err := contract.Parse(data)
		if err != nil {
			t.Fatalf("Parse(%s) error: %v", name, err)
		}
		key := contract.IdempotencyKey(dm)
		if prev, ok := keys[key]; ok {
			t.Errorf("key collision: %s and %s produce same key %s", prev, name, key)
		}
		keys[key] = name
	}
}

// ──────────────────────────────────────────────
// Group 4: Round-Trip (Marshal → Parse → Compare)
// Marshal a DMail, parse it back, verify fields match.
// ──────────────────────────────────────────────

func TestGroup4_RoundTrip(t *testing.T) {
	cases := []contract.DMail{
		{
			SchemaVersion: "1",
			Name:          "roundtrip-feedback",
			Kind:          "design-feedback",
			Description:   "Round-trip test feedback",
			Issues:        []string{"TEST-1"},
			Severity:      "medium",
			Body:          "This is the body.\n",
		},
		{
			SchemaVersion: "1",
			Name:          "roundtrip-convergence",
			Kind:          "convergence",
			Description:   "Round-trip convergence with targets",
			Targets:       []string{"auth", "session"},
			Body:          "Convergence body.\n",
		},
		{
			SchemaVersion: "1",
			Name:          "roundtrip-minimal",
			Kind:          "report",
			Description:   "Minimal round-trip",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// marshal
			data, err := contract.Marshal(tc)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			// parse
			got, err := contract.Parse(data)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// compare core fields
			if got.SchemaVersion != tc.SchemaVersion {
				t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, tc.SchemaVersion)
			}
			if got.Name != tc.Name {
				t.Errorf("Name = %q, want %q", got.Name, tc.Name)
			}
			if got.Kind != tc.Kind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.Kind)
			}
			if got.Description != tc.Description {
				t.Errorf("Description = %q, want %q", got.Description, tc.Description)
			}
			if got.Severity != tc.Severity {
				t.Errorf("Severity = %q, want %q", got.Severity, tc.Severity)
			}
			if len(got.Issues) != len(tc.Issues) {
				t.Errorf("Issues len = %d, want %d", len(got.Issues), len(tc.Issues))
			}
			if len(got.Targets) != len(tc.Targets) {
				t.Errorf("Targets len = %d, want %d", len(got.Targets), len(tc.Targets))
			}
			// Body comparison: trim trailing whitespace for resilience
			wantBody := strings.TrimRight(tc.Body, "\n")
			gotBody := strings.TrimRight(got.Body, "\n")
			if gotBody != wantBody {
				t.Errorf("Body = %q, want %q", gotBody, wantBody)
			}
		})
	}
}

// ──────────────────────────────────────────────
// Group 5: Postel Edge Cases
// Unknown kinds and future schema versions should parse but differ from v1.
// ──────────────────────────────────────────────

func TestGroup5_PostelUnknownKind(t *testing.T) {
	data := readGolden(t, "unknown-kind.md")
	dm, err := contract.Parse(data)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if dm.Kind != "advisory" {
		t.Errorf("Kind = %q, want %q", dm.Kind, "advisory")
	}
	// Should parse but is NOT a valid schema v1 kind
	validKinds := map[string]bool{
		"specification": true, "report": true,
		"design-feedback": true, "implementation-feedback": true, "convergence": true,
	}
	if validKinds[dm.Kind] {
		t.Error("unknown kind should not be in valid schema v1 kinds")
	}
}

func TestGroup5_PostelFutureSchema(t *testing.T) {
	data := readGolden(t, "future-schema.md")
	dm, err := contract.Parse(data)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if dm.SchemaVersion != "2" {
		t.Errorf("SchemaVersion = %q, want %q", dm.SchemaVersion, "2")
	}
	// Core fields should still parse
	if dm.Name == "" {
		t.Error("name should parse even with future schema version")
	}
}

// ──────────────────────────────────────────────
// Group 6: Targets Field (amadeus-specific)
// Verify the targets field is preserved through parse.
// ──────────────────────────────────────────────

func TestGroup6_TargetsField(t *testing.T) {
	data := readGolden(t, "amadeus-convergence.md")
	dm, err := contract.Parse(data)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(dm.Targets) != 2 {
		t.Fatalf("Targets len = %d, want 2", len(dm.Targets))
	}
	if dm.Targets[0] != "authentication" {
		t.Errorf("Targets[0] = %q, want %q", dm.Targets[0], "authentication")
	}
	if dm.Targets[1] != "session-management" {
		t.Errorf("Targets[1] = %q, want %q", dm.Targets[1], "session-management")
	}
}

// ──────────────────────────────────────────────
// Group 7: JSON Schema Validation
// Tool-produced golden files must validate against the v1 JSON Schema.
// Edge cases (unknown kind, future schema) must fail schema validation
// but still parse successfully (Postel's Law in action).
// ──────────────────────────────────────────────

const schemaPath = "testdata/schema/dmail-frontmatter.v1.schema.json"

// compileSchema loads and compiles the D-Mail v1 JSON Schema.
func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	sch, err := jsonschema.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

// frontmatterToValidatable converts raw D-Mail bytes to a JSON-compatible
// value suitable for JSON Schema validation. YAML frontmatter is extracted,
// round-tripped through JSON to normalize types (YAML int → JSON number).
func frontmatterToValidatable(t *testing.T, data []byte) any {
	t.Helper()
	m, err := contract.ParseFrontmatterMap(data)
	if err != nil {
		t.Fatalf("parse frontmatter map: %v", err)
	}
	// Round-trip through JSON to ensure types match JSON Schema expectations.
	// YAML might infer integers for unquoted numbers; JSON Schema expects
	// the types declared in the schema (string for dmail-schema-version).
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal to JSON: %v", err)
	}
	var v any
	if err := json.Unmarshal(jsonBytes, &v); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	return v
}

func TestGroup7_JSONSchemaToolFilesValid(t *testing.T) {
	sch := compileSchema(t)

	toolFiles := []string{
		"sightjack-spec.md",
		"sightjack-report.md",
		"sightjack-feedback.md",
		"paintress-report.md",
		"amadeus-feedback-high.md",
		"amadeus-convergence.md",
		"minimal.md",
	}
	for _, name := range toolFiles {
		t.Run(name, func(t *testing.T) {
			data := readGolden(t, name)
			v := frontmatterToValidatable(t, data)
			if err := sch.Validate(v); err != nil {
				t.Errorf("schema validation failed: %v", err)
			}
		})
	}
}

func TestGroup7_JSONSchemaRejectsEdgeCases(t *testing.T) {
	sch := compileSchema(t)

	cases := []struct {
		file   string
		reason string
	}{
		{"unknown-kind.md", "kind 'advisory' not in schema v1 enum"},
		{"future-schema.md", "dmail-schema-version '2' does not match const '1'"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data := readGolden(t, tc.file)
			v := frontmatterToValidatable(t, data)
			if err := sch.Validate(v); err == nil {
				t.Errorf("expected schema validation to fail (%s), but it passed", tc.reason)
			}
		})
	}
}
