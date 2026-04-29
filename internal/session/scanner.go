package session

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
	"gopkg.in/yaml.v3"
)

// Skill directory names for D-Mail capabilities.
const (
	SkillSendable = "dmail-sendable"
	SkillReadable = "dmail-readable"
)

// DMailCapability represents a single D-Mail kind declaration.
type DMailCapability struct { // nosemgrep: structure.multiple-exported-structs-go -- scanner family (DMailCapability/SkillMetadata/SkillFrontmatter) is cohesive SKILL.md parsing type set [permanent]
	Kind        string `yaml:"kind"`
	Description string `yaml:"description"`
}

// SkillMetadata holds D-Mail extension fields within SKILL.md metadata.
type SkillMetadata struct { // nosemgrep: structure.multiple-exported-structs-go -- scanner family; see DMailCapability [permanent]
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

// ParseSkillFrontmatter extracts YAML frontmatter from a SKILL.md file.
// The frontmatter must be delimited by "---" lines.
// D-Mail capabilities must be declared under metadata with dmail-schema-version: "1".
func ParseSkillFrontmatter(data []byte) (*SkillFrontmatter, error) {
	content := string(data)

	// Find frontmatter delimiters
	if !strings.HasPrefix(content, "---") {
		return nil, errors.New("no YAML frontmatter found: file must start with ---")
	}

	// Find the closing ---
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
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
		if skill.Metadata.SchemaVersion != domain.SupportedDMailSchemaVersion {
			return nil, fmt.Errorf("unsupported dmail-schema-version %q: only \"1\" is supported", skill.Metadata.SchemaVersion)
		}
		skill.Produces = skill.Metadata.Produces
		skill.Consumes = skill.Metadata.Consumes
	}

	// Validate all declared kinds
	for _, p := range skill.Produces {
		if _, err := domain.ParseDMailKind(p.Kind); err != nil {
			return nil, fmt.Errorf("produces: %w", err)
		}
	}
	for _, c := range skill.Consumes {
		if _, err := domain.ParseDMailKind(c.Kind); err != nil {
			return nil, fmt.Errorf("consumes: %w", err)
		}
	}

	return &skill, nil
}

// ScanRepository scans a repository path for dot-directories containing
// D-Mail skill declarations (skills/dmail-sendable/SKILL.md and
// skills/dmail-readable/SKILL.md).
func ScanRepository(repoPath string) ([]domain.Endpoint, error) {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, err
	}

	var endpoints []domain.Endpoint
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only consider dot-prefixed directories
		if !strings.HasPrefix(name, ".") {
			continue
		}
		// Skip common non-tool dot directories
		if name == ".git" || name == ".github" || name == domain.StateDir {
			continue
		}

		ep, found, err := scanEndpoint(repoPath, name)
		if err != nil {
			return nil, err
		}
		if found {
			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, nil
}

// scanEndpoint checks a single dot-directory for D-Mail skill declarations.
func scanEndpoint(repoPath, dirName string) (domain.Endpoint, bool, error) {
	ep := domain.Endpoint{Dir: dirName}
	found := false

	// Check for sendable skill
	sendablePath := filepath.Join(repoPath, dirName, "skills", SkillSendable, "SKILL.md")
	if data, err := os.ReadFile(sendablePath); err == nil {
		skill, err := ParseSkillFrontmatter(data)
		if err != nil {
			return ep, false, err
		}
		for _, p := range skill.Produces {
			ep.Produces = append(ep.Produces, p.Kind)
		}
		found = true
	}

	// Check for readable skill
	readablePath := filepath.Join(repoPath, dirName, "skills", SkillReadable, "SKILL.md")
	if data, err := os.ReadFile(readablePath); err == nil {
		skill, err := ParseSkillFrontmatter(data)
		if err != nil {
			return ep, false, err
		}
		for _, c := range skill.Consumes {
			ep.Consumes = append(ep.Consumes, c.Kind)
		}
		found = true
	}

	return ep, found, nil
}
