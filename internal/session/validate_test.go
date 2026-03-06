package session
// white-box-reason: session internals: tests unexported skillsRefAvailable check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
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

func TestValidateEndpointSkills_EmptyDeclarations(t *testing.T) {
	if !skillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — endpoint with empty produces/consumes but SKILL.md exists on disk
	repoDir := t.TempDir()
	sendableDir := filepath.Join(repoDir, ".siren", "skills", "dmail-sendable")
	if err := os.MkdirAll(sendableDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Non-compliant SKILL.md (top-level produces)
	if err := os.WriteFile(filepath.Join(sendableDir, "SKILL.md"), []byte(`---
name: dmail-sendable
description: test
produces:
  - kind: specification
---
`), 0644); err != nil {
		t.Fatal(err)
	}

	ep := domain.EndpointConfig{
		Dir:      ".siren",
		Produces: nil, // empty — intentional empty
		Consumes: nil,
	}

	// when
	warnings := validateEndpointSkills(repoDir, ep)

	// then — should still validate because SKILL.md exists on disk
	if len(warnings) == 0 {
		t.Error("expected skills-ref warnings for non-compliant SKILL.md even with empty declarations")
	}
}

func TestWalkUpForSkillsRef_FindsSubmodule(t *testing.T) {
	// given — a directory tree with skills-ref/skills-ref/pyproject.toml
	root := t.TempDir()
	submodDir := filepath.Join(root, "skills-ref", "skills-ref")
	if err := os.MkdirAll(submodDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(submodDir, "pyproject.toml"), []byte("[project]\nname = \"skills-ref\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Start from a nested directory
	nested := filepath.Join(root, "deep", "nested", "dir")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	// when
	found := walkUpForSkillsRef(nested)

	// then
	if found != submodDir {
		t.Errorf("walkUpForSkillsRef(%q) = %q, want %q", nested, found, submodDir)
	}
}

func TestWalkUpForSkillsRef_ReturnsEmptyOutsideRepo(t *testing.T) {
	// given — a temp directory with no skills-ref submodule
	isolated := t.TempDir()

	// when
	found := walkUpForSkillsRef(isolated)

	// then
	if found != "" {
		t.Errorf("walkUpForSkillsRef(%q) = %q, want empty", isolated, found)
	}
}

func TestFindSkillsRefDir_EnvVarOverride(t *testing.T) {
	// given — explicit PHONEWAVE_SKILLS_REF pointing to a valid directory
	submodDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(submodDir, "pyproject.toml"), []byte("[project]\nname = \"skills-ref\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PHONEWAVE_SKILLS_REF", submodDir)

	// when
	found := findSkillsRefDir()

	// then
	if found != submodDir {
		t.Errorf("findSkillsRefDir() = %q, want %q (from env var)", found, submodDir)
	}
}

func TestFindSkillsRefDir_EnvVarInvalidIgnored(t *testing.T) {
	// given — PHONEWAVE_SKILLS_REF pointing to nonexistent path
	t.Setenv("PHONEWAVE_SKILLS_REF", "/nonexistent/path")

	// when — should fall through to other discovery methods (not panic)
	_ = findSkillsRefDir()
	// then — no crash; result depends on CWD/executable fallbacks
}

func skillsRefAvailable() bool {
	_, cancel, err := skillsRefCommand(os.DevNull)
	if err == nil {
		cancel()
	}
	return err == nil
}
