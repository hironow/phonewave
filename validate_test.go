package phonewave

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSkillDir_ValidSkill(t *testing.T) {
	if !skillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — a valid SKILL.md with Agent Skills compliant frontmatter
	skillDir := t.TempDir()
	skillMD := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte(`---
name: dmail-sendable
description: Produces D-Mail messages to outbox for phonewave delivery.
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
---

# dmail-sendable
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Rename dir to match skill name (Agent Skills spec requires dir name == name)
	namedDir := filepath.Join(filepath.Dir(skillDir), "dmail-sendable")
	if err := os.Rename(skillDir, namedDir); err != nil {
		t.Fatal(err)
	}

	// when
	problems, err := ValidateSkillDir(namedDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("expected no problems, got: %v", problems)
	}
}

func TestValidateSkillDir_InvalidSkill(t *testing.T) {
	if !skillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — SKILL.md with top-level produces (non-compliant)
	skillDir := t.TempDir()
	skillMD := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte(`---
name: dmail-sendable
description: Produces D-Mail messages to outbox.
produces:
  - kind: specification
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	namedDir := filepath.Join(filepath.Dir(skillDir), "dmail-sendable")
	if err := os.Rename(skillDir, namedDir); err != nil {
		t.Fatal(err)
	}

	// when
	problems, err := ValidateSkillDir(namedDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(problems) == 0 {
		t.Error("expected validation problems for non-compliant SKILL.md, got none")
	}
}

func TestParseValidationOutput(t *testing.T) {
	output := `Validation failed for /path/to/skill:
  - Unexpected fields in frontmatter: produces. Only ['allowed-tools', 'compatibility', 'description', 'license', 'metadata', 'name'] are allowed.
  - Name must match parent directory name.
`
	problems := parseValidationOutput(output)
	if len(problems) != 2 {
		t.Fatalf("want 2 problems, got %d: %v", len(problems), problems)
	}
}

func skillsRefAvailable() bool {
	_, err := skillsRefCommand("/dev/null")
	return err == nil
}
