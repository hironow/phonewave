package phonewave

import (
	"testing"
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

func TestParseFrontmatter_RejectsTopLevelWithoutMetadata(t *testing.T) {
	content := `---
name: "dmail-sendable"
description: "Uses top-level produces without metadata"
produces:
  - kind: specification
---
`
	_, err := ParseSkillFrontmatter([]byte(content))
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

func TestParseFrontmatter_RejectsMixedTopLevelAndMetadata(t *testing.T) {
	// given — SKILL.md with top-level produces AND metadata.dmail-schema-version
	// but metadata.produces is absent -> top-level would be silently dropped
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
	_, err := ParseSkillFrontmatter([]byte(content))

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
	_, err := ParseSkillFrontmatter([]byte(content))

	// then — should fail fast, not silently drop capabilities
	if err == nil {
		t.Fatal("expected error for metadata capabilities without dmail-schema-version, got nil")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Just a markdown file without frontmatter`
	_, err := ParseSkillFrontmatter([]byte(content))
	if err == nil {
		t.Fatal("expected error for missing frontmatter, got nil")
	}
}
