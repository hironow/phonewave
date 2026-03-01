// Package contract provides a minimal, Postel-liberal D-Mail parser for
// cross-tool contract testing. It accepts any YAML frontmatter with
// "---" delimiters regardless of schema version or unknown fields.
//
// This parser is intentionally more permissive than any individual tool's
// parser, following ADR S0021 (Postel's Law): be liberal in what you accept.
package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DMail is the contract-level representation of a D-Mail.
// All fields are strings to maximize parse compatibility.
type DMail struct {
	SchemaVersion string            `yaml:"dmail-schema-version"`
	Name          string            `yaml:"name"`
	Kind          string            `yaml:"kind"`
	Description   string            `yaml:"description"`
	Issues        []string          `yaml:"issues,omitempty"`
	Severity      string            `yaml:"severity,omitempty"`
	Targets       []string          `yaml:"targets,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	Body          string            `yaml:"-"`
}

// Parse parses a D-Mail from raw bytes. It accepts any valid YAML
// frontmatter delimited by "---" lines, regardless of field values.
func Parse(data []byte) (DMail, error) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return DMail{}, fmt.Errorf("missing opening frontmatter delimiter")
	}
	rest := s[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		// Handle file ending with "---" and no trailing newline
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			return DMail{}, fmt.Errorf("missing closing frontmatter delimiter")
		}
	}
	yamlPart := rest[:idx]
	bodyPart := rest[idx+5:] // skip "\n---\n"

	var dm DMail
	if err := yaml.Unmarshal([]byte(yamlPart), &dm); err != nil {
		return DMail{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	dm.Body = strings.TrimLeft(bodyPart, "\n")
	return dm, nil
}

// Marshal serializes a DMail to YAML frontmatter + Markdown body format.
func Marshal(dm DMail) ([]byte, error) {
	var buf strings.Builder
	buf.WriteString("---\n")
	yamlData, err := yaml.Marshal(dm)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}
	buf.Write(yamlData)
	buf.WriteString("---\n")
	if dm.Body != "" {
		buf.WriteString("\n")
		buf.WriteString(dm.Body)
		if !strings.HasSuffix(dm.Body, "\n") {
			buf.WriteString("\n")
		}
	}
	return []byte(buf.String()), nil
}

// ParseFrontmatterMap extracts YAML frontmatter as a generic map.
// Used for JSON Schema validation where typed struct fields are insufficient.
func ParseFrontmatterMap(data []byte) (map[string]any, error) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return nil, fmt.Errorf("missing opening frontmatter delimiter")
	}
	rest := s[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			return nil, fmt.Errorf("missing closing frontmatter delimiter")
		}
	}
	yamlPart := rest[:idx]

	var m map[string]any
	if err := yaml.Unmarshal([]byte(yamlPart), &m); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	return m, nil
}

// IdempotencyKey computes the SHA256 content-based key from core fields.
// This matches the algorithm used by all 4 tools:
// SHA256(name + \x00 + kind + \x00 + description + \x00 + body)
func IdempotencyKey(dm DMail) string {
	h := sha256.New()
	h.Write([]byte(dm.Name))
	h.Write([]byte{0})
	h.Write([]byte(dm.Kind))
	h.Write([]byte{0})
	h.Write([]byte(dm.Description))
	h.Write([]byte{0})
	h.Write([]byte(dm.Body))
	return hex.EncodeToString(h.Sum(nil))
}
