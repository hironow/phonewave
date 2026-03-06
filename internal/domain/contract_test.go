//go:build contract

package domain
// white-box-reason: contract validation: tests unexported golden file enumeration (parseDMailFrontmatter)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const contractGoldenDir = "../../testdata/contract"

func contractGoldenFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(contractGoldenDir)
	if err != nil {
		t.Fatalf("read contract golden dir: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		t.Fatal("no contract golden files found")
	}
	return files
}

func readContractGolden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractGoldenDir, name))
	if err != nil {
		t.Fatalf("read contract golden %s: %v", name, err)
	}
	return data
}

// TestContract_ParseDMailFrontmatter verifies that phonewave's internal
// parser can extract frontmatter from all cross-tool golden files.
// parseDMailFrontmatter is Postel-liberal: it does not validate field values.
func TestContract_ParseDMailFrontmatter(t *testing.T) {
	for _, name := range contractGoldenFiles(t) {
		t.Run(name, func(t *testing.T) {
			data := readContractGolden(t, name)
			fm, err := parseDMailFrontmatter(data)
			if err != nil {
				t.Fatalf("parseDMailFrontmatter error: %v", err)
			}
			if fm.Name == "" {
				t.Error("parsed name is empty")
			}
			if fm.Kind == "" {
				t.Error("parsed kind is empty")
			}
			if fm.Description == "" {
				t.Error("parsed description is empty")
			}
			if fm.SchemaVersion == "" {
				t.Error("parsed schema version is empty")
			}
		})
	}
}

// TestContract_ExtractDMailKind verifies that phonewave's strict kind
// extraction succeeds for valid schema v1 D-Mails. This is the "send
// strict" side — phonewave validates kind and schema version.
func TestContract_ExtractDMailKind(t *testing.T) {
	validFiles := []string{
		"sightjack-spec.md",
		"sightjack-report.md",
		"sightjack-feedback.md",
		"paintress-report.md",
		"amadeus-feedback-high.md",
		"amadeus-convergence.md",
		"minimal.md",
	}
	for _, name := range validFiles {
		t.Run(name, func(t *testing.T) {
			data := readContractGolden(t, name)
			kind, err := ExtractDMailKind(data)
			if err != nil {
				t.Fatalf("ExtractDMailKind error: %v", err)
			}
			if kind == "" {
				t.Error("extracted kind is empty")
			}
		})
	}
}

// TestContract_ExtractDMailKindRejectsEdgeCases verifies that phonewave's
// strict validation rejects D-Mails with unknown kinds or future schemas.
func TestContract_ExtractDMailKindRejectsEdgeCases(t *testing.T) {
	cases := []struct {
		file   string
		reason string
	}{
		{"unknown-kind.md", "kind 'advisory' not in valid schema v1 kinds"},
		{"future-schema.md", "dmail-schema-version '2' not supported"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data := readContractGolden(t, tc.file)
			_, err := ExtractDMailKind(data)
			if err == nil {
				t.Errorf("expected ExtractDMailKind to fail (%s), but it passed", tc.reason)
			}
		})
	}
}
