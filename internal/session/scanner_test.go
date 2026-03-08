package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestParseFrontmatter_Produces(t *testing.T) {
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

This skill produces D-Mail messages.
`
	skill, err := session.ParseSkillFrontmatter([]byte(content))
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
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  consumes:
    - kind: design-feedback
      description: "Corrective feedback from verifier"
    - kind: specification
      description: "Issue specification"
---
`
	skill, err := session.ParseSkillFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.Consumes) != 2 {
		t.Fatalf("want 2 consumes, got %d", len(skill.Consumes))
	}
	if skill.Consumes[0].Kind != "design-feedback" {
		t.Errorf("consumes[0].kind = %q, want %q", skill.Consumes[0].Kind, "design-feedback")
	}
	if skill.Consumes[1].Kind != "specification" {
		t.Errorf("consumes[1].kind = %q, want %q", skill.Consumes[1].Kind, "specification")
	}
}

func TestParseFrontmatter_RejectsTopLevelWithoutMetadata(t *testing.T) {
	content := `---
name: "dmail-sendable"
description: "Uses top-level produces without metadata"
produces:
  - kind: specification
---
`
	_, err := session.ParseSkillFrontmatter([]byte(content))
	if err == nil {
		t.Fatal("expected error for top-level produces without dmail-schema-version, got nil")
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
	skill, err := session.ParseSkillFrontmatter([]byte(content))
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
    - kind: design-feedback
      description: "Corrective feedback from verifier"
    - kind: specification
      description: "Issue specification"
---
`
	skill, err := session.ParseSkillFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.Consumes) != 2 {
		t.Fatalf("want 2 consumes, got %d", len(skill.Consumes))
	}
	if skill.Consumes[0].Kind != "design-feedback" {
		t.Errorf("consumes[0].kind = %q, want %q", skill.Consumes[0].Kind, "design-feedback")
	}
	if skill.Consumes[1].Kind != "specification" {
		t.Errorf("consumes[1].kind = %q, want %q", skill.Consumes[1].Kind, "specification")
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
	_, err := session.ParseSkillFrontmatter([]byte(content))
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
	_, err := session.ParseSkillFrontmatter([]byte(content))
	if err == nil {
		t.Fatal("expected error for unsupported dmail-schema-version \"2\", got nil")
	}
}

func TestParseFrontmatter_RejectsMixedTopLevelAndMetadata(t *testing.T) {
	// given — SKILL.md with top-level produces AND metadata.dmail-schema-version
	// but metadata.produces is absent → top-level would be silently dropped
	content := `---
name: "dmail-sendable"
description: "Mixed format"
produces:
  - kind: specification
metadata:
  dmail-schema-version: "1"
---
`
	// when
	_, err := session.ParseSkillFrontmatter([]byte(content))

	// then — should reject: top-level capabilities must not coexist with metadata
	if err == nil {
		t.Fatal("expected error for mixed top-level and metadata capabilities, got nil")
	}
}

func TestParseFrontmatter_RejectsMetadataCapabilitiesWithoutSchemaVersion(t *testing.T) {
	// given — metadata.produces present but dmail-schema-version missing
	content := `---
name: "dmail-sendable"
description: "Forgot schema version"
metadata:
  produces:
    - kind: specification
---
`
	// when
	_, err := session.ParseSkillFrontmatter([]byte(content))

	// then — should fail fast, not silently drop capabilities
	if err == nil {
		t.Fatal("expected error for metadata capabilities without dmail-schema-version, got nil")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Just a markdown file without frontmatter`
	_, err := session.ParseSkillFrontmatter([]byte(content))
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
description: "Produces D-Mail messages"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
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
description: "Reads D-Mail messages"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  consumes:
    - kind: design-feedback
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
description: "Produces implementation reports"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
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
description: "Reads D-Mail messages"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  consumes:
    - kind: specification
      description: "Issue specification"
    - kind: design-feedback
      description: "Corrective feedback"
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	endpoints, err := session.ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("want 2 endpoints, got %d", len(endpoints))
	}

	// Find endpoints by dir name
	endpointMap := make(map[string]domain.Endpoint)
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
	if len(siren.Consumes) != 1 || siren.Consumes[0] != "design-feedback" {
		t.Errorf(".siren consumes = %v, want [design-feedback]", siren.Consumes)
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
	endpoints, err := session.ScanRepository(repoDir)

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
name: "dmail-sendable"
description: "test"
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	endpoints, err := session.ScanRepository(repoDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(endpoints) != 0 {
		t.Errorf("want 0 endpoints (non-dot dirs should be skipped), got %d", len(endpoints))
	}
}
