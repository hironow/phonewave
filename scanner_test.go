package phonewave

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter_Produces(t *testing.T) {
	content := `---
name: "dmail-sendable"
description: "Produces D-Mail messages to outbox"
produces:
  - kind: specification
    description: "Issue specification ready for implementation"
---

# Sendable Skill

This skill produces D-Mail messages.
`
	skill, err := ParseSkillFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.Produces) != 1 {
		t.Fatalf("want 1 produces, got %d", len(skill.Produces))
	}
	if skill.Produces[0].Kind != "specification" {
		t.Errorf("produces[0].kind = %q, want %q", skill.Produces[0].Kind, "specification")
	}
	if len(skill.Consumes) != 0 {
		t.Errorf("want 0 consumes, got %d", len(skill.Consumes))
	}
}

func TestParseFrontmatter_Consumes(t *testing.T) {
	content := `---
name: "dmail-readable"
description: "Reads D-Mail messages from inbox"
consumes:
  - kind: feedback
    description: "Corrective feedback from verifier"
  - kind: specification
    description: "Issue specification"
---
`
	skill, err := ParseSkillFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.Consumes) != 2 {
		t.Fatalf("want 2 consumes, got %d", len(skill.Consumes))
	}
	if skill.Consumes[0].Kind != "feedback" {
		t.Errorf("consumes[0].kind = %q, want %q", skill.Consumes[0].Kind, "feedback")
	}
	if skill.Consumes[1].Kind != "specification" {
		t.Errorf("consumes[1].kind = %q, want %q", skill.Consumes[1].Kind, "specification")
	}
}

func TestParseFrontmatter_MetadataProduces(t *testing.T) {
	content := `---
name: "dmail-sendable"
description: "Produces D-Mail messages to outbox"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
      description: "Issue specification ready for implementation"
---

# Sendable Skill
`
	skill, err := ParseSkillFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.Produces) != 1 {
		t.Fatalf("want 1 produces, got %d", len(skill.Produces))
	}
	if skill.Produces[0].Kind != "specification" {
		t.Errorf("produces[0].kind = %q, want %q", skill.Produces[0].Kind, "specification")
	}
	if skill.Metadata.SchemaVersion != "1" {
		t.Errorf("metadata.dmail-schema-version = %q, want %q", skill.Metadata.SchemaVersion, "1")
	}
}

func TestParseFrontmatter_MetadataConsumes(t *testing.T) {
	content := `---
name: "dmail-readable"
description: "Reads D-Mail messages from inbox"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  consumes:
    - kind: feedback
      description: "Corrective feedback from verifier"
    - kind: specification
      description: "Issue specification"
---
`
	skill, err := ParseSkillFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.Consumes) != 2 {
		t.Fatalf("want 2 consumes, got %d", len(skill.Consumes))
	}
	if skill.Consumes[0].Kind != "feedback" {
		t.Errorf("consumes[0].kind = %q, want %q", skill.Consumes[0].Kind, "feedback")
	}
	if skill.Consumes[1].Kind != "specification" {
		t.Errorf("consumes[1].kind = %q, want %q", skill.Consumes[1].Kind, "specification")
	}
}

func TestParseFrontmatter_MetadataEmptyOverridesLegacy(t *testing.T) {
	// metadata.produces: [] should override legacy top-level produces
	content := `---
name: "dmail-sendable"
description: "Migrated to metadata with empty produces"
produces:
  - kind: specification
metadata:
  dmail-schema-version: "1"
  produces: []
---
`
	skill, err := ParseSkillFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.Produces) != 0 {
		t.Errorf("want 0 produces (metadata override), got %d: %v", len(skill.Produces), skill.Produces)
	}
}

func TestParseFrontmatter_MetadataValidatesKind(t *testing.T) {
	content := `---
name: "dmail-sendable"
description: "Bad kind"
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: invalid_kind
---
`
	_, err := ParseSkillFrontmatter([]byte(content))
	if err == nil {
		t.Fatal("expected error for invalid kind in metadata, got nil")
	}
}

func TestParseFrontmatter_UnsupportedSchemaVersion(t *testing.T) {
	content := `---
name: "dmail-sendable"
description: "Future schema"
metadata:
  dmail-schema-version: "2"
  produces:
    - kind: specification
---
`
	_, err := ParseSkillFrontmatter([]byte(content))
	if err == nil {
		t.Fatal("expected error for unsupported dmail-schema-version \"2\", got nil")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Just a markdown file without frontmatter`
	_, err := ParseSkillFrontmatter([]byte(content))
	if err == nil {
		t.Fatal("expected error for missing frontmatter, got nil")
	}
}

func TestScanRepository_DiscoverEndpoints(t *testing.T) {
	// given — set up a fake repository with .siren and .expedition
	repoDir := t.TempDir()

	// .siren with sendable skill
	sirenSendable := filepath.Join(repoDir, ".siren", "skills", "dmail-sendable")
	if err := os.MkdirAll(sirenSendable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sirenSendable, "SKILL.md"), []byte(`---
name: "dmail-sendable"
produces:
  - kind: specification
    description: "Issue specification"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// .siren with readable skill
	sirenReadable := filepath.Join(repoDir, ".siren", "skills", "dmail-readable")
	if err := os.MkdirAll(sirenReadable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sirenReadable, "SKILL.md"), []byte(`---
name: "dmail-readable"
consumes:
  - kind: feedback
    description: "Corrective feedback"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// .expedition with sendable
	expedSendable := filepath.Join(repoDir, ".expedition", "skills", "dmail-sendable")
	if err := os.MkdirAll(expedSendable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expedSendable, "SKILL.md"), []byte(`---
name: "dmail-sendable"
produces:
  - kind: report
    description: "Implementation report"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// .expedition with readable
	expedReadable := filepath.Join(repoDir, ".expedition", "skills", "dmail-readable")
	if err := os.MkdirAll(expedReadable, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expedReadable, "SKILL.md"), []byte(`---
name: "dmail-readable"
consumes:
  - kind: specification
    description: "Issue specification"
  - kind: feedback
    description: "Corrective feedback"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	endpoints, err := ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(endpoints))
	}

	// Find endpoints by dir name
	endpointMap := make(map[string]Endpoint)
	for _, ep := range endpoints {
		endpointMap[ep.Dir] = ep
	}

	siren, ok := endpointMap[".siren"]
	if !ok {
		t.Fatal("missing .siren endpoint")
	}
	if len(siren.Produces) != 1 || siren.Produces[0] != "specification" {
		t.Errorf(".siren produces = %v, want [specification]", siren.Produces)
	}
	if len(siren.Consumes) != 1 || siren.Consumes[0] != "feedback" {
		t.Errorf(".siren consumes = %v, want [feedback]", siren.Consumes)
	}

	exped, ok := endpointMap[".expedition"]
	if !ok {
		t.Fatal("missing .expedition endpoint")
	}
	if len(exped.Produces) != 1 || exped.Produces[0] != "report" {
		t.Errorf(".expedition produces = %v, want [report]", exped.Produces)
	}
	if len(exped.Consumes) != 2 {
		t.Errorf(".expedition consumes = %v, want [specification feedback]", exped.Consumes)
	}
}

func TestScanRepository_NoDotDirs(t *testing.T) {
	// given — empty repo
	repoDir := t.TempDir()

	// when
	endpoints, err := ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 0 {
		t.Errorf("want 0 endpoints, got %d", len(endpoints))
	}
}

func TestScanRepository_SkipsNonDotDirs(t *testing.T) {
	// given — a regular directory that happens to have skills/
	repoDir := t.TempDir()
	regularSkills := filepath.Join(repoDir, "src", "skills", "dmail-sendable")
	if err := os.MkdirAll(regularSkills, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regularSkills, "SKILL.md"), []byte(`---
produces:
  - kind: specification
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	endpoints, err := ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 0 {
		t.Errorf("want 0 endpoints (non-dot dirs should be skipped), got %d", len(endpoints))
	}
}
