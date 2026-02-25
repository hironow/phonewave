package phonewave

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Skill directory names for D-Mail capabilities.
const (
	SkillSendable = "dmail-sendable"
	SkillReadable = "dmail-readable"
)

// DMailCapability represents a single D-Mail kind declaration.
type DMailCapability struct {
	Kind        string `yaml:"kind"`
	Description string `yaml:"description"`
}

// SkillMetadata holds D-Mail extension fields within SKILL.md metadata.
type SkillMetadata struct {
	SchemaVersion string            `yaml:"dmail-schema-version"`
	Produces      []DMailCapability `yaml:"produces"`
	Consumes      []DMailCapability `yaml:"consumes"`
}

// SkillFrontmatter holds parsed YAML frontmatter from a SKILL.md file.
type SkillFrontmatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Produces    []DMailCapability `yaml:"produces"`
	Consumes    []DMailCapability `yaml:"consumes"`
	Metadata    SkillMetadata     `yaml:"metadata"`
}

// Endpoint represents a discovered tool endpoint within a repository.
type Endpoint struct {
	Dir      string   // dot-directory name, e.g. ".siren"
	Produces []string // list of kind values this endpoint produces
	Consumes []string // list of kind values this endpoint consumes
}

// ParseSkillFrontmatter extracts YAML frontmatter from a SKILL.md file.
// The frontmatter must be delimited by "---" lines.
// D-Mail capabilities must be declared under metadata with dmail-schema-version: "1".
func ParseSkillFrontmatter(data []byte) (*SkillFrontmatter, error) {
	content := string(data)

	// Find frontmatter delimiters
	if len(content) < 3 || content[:3] != "---" {
		return nil, errors.New("no YAML frontmatter found: file must start with ---")
	}

	// Find the closing ---
	rest := content[3:]
	idx := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+3 < len(rest) && rest[i+1:i+4] == "---" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, errors.New("no closing --- found for YAML frontmatter")
	}

	frontmatter := rest[:idx]

	var skill SkillFrontmatter
	if err := yaml.NewDecoder(bytes.NewReader([]byte(frontmatter))).Decode(&skill); err != nil {
		return nil, err
	}

	// Reject top-level produces/consumes — capabilities must be under metadata.
	if len(skill.Produces) > 0 || len(skill.Consumes) > 0 {
		return nil, errors.New("top-level produces/consumes is not supported; use metadata with dmail-schema-version: \"1\"")
	}

	// Reject metadata capabilities without schema version.
	if skill.Metadata.SchemaVersion == "" {
		if len(skill.Metadata.Produces) > 0 || len(skill.Metadata.Consumes) > 0 {
			return nil, errors.New("metadata contains produces/consumes but missing required dmail-schema-version")
		}
	} else {
		// Read capabilities from metadata when schema version is declared.
		if skill.Metadata.SchemaVersion != SupportedDMailSchemaVersion {
			return nil, fmt.Errorf("unsupported dmail-schema-version %q: only \"1\" is supported", skill.Metadata.SchemaVersion)
		}
		skill.Produces = skill.Metadata.Produces
		skill.Consumes = skill.Metadata.Consumes
	}

	// Validate all declared kinds
	for _, p := range skill.Produces {
		if err := ValidateKind(p.Kind); err != nil {
			return nil, fmt.Errorf("produces: %w", err)
		}
	}
	for _, c := range skill.Consumes {
		if err := ValidateKind(c.Kind); err != nil {
			return nil, fmt.Errorf("consumes: %w", err)
		}
	}

	return &skill, nil
}
