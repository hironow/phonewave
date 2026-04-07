package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
)

// IndexEntry represents one line in the archive index JSONL file.
type IndexEntry struct {
	Timestamp string `json:"ts"`
	Operation string `json:"op"`
	Issue     string `json:"issue"`
	Status    string `json:"status"`
	Tool      string `json:"tool"`
	Path      string `json:"path"`
	Summary   string `json:"summary"`
}

// ErrorMetadata holds metadata for a failed D-Mail stored as a .err sidecar.
type ErrorMetadata struct {
	SourceOutbox string    `yaml:"source_outbox"`
	Kind         DMailKind `yaml:"kind"`
	OriginalName string    `yaml:"original_name"`
	Attempts     int       `yaml:"attempts"`
	Error        string    `yaml:"error"`
	Timestamp    time.Time `yaml:"timestamp"`
}

// SupportedDMailSchemaVersion is the only accepted dmail-schema-version value.
const SupportedDMailSchemaVersion = "1"

// UnknownKind is the canonical fallback when a D-Mail's kind cannot be determined.
const UnknownKind = "unknown"

// DeliveryFlushed represents a single target that was successfully flushed.
type DeliveryFlushed struct {
	DMailPath string
	Target    string
}

// StagedDelivery represents an unflushed delivery intent.
type StagedDelivery struct {
	DMailPath string
	Target    string
	Data      []byte
}

// DMailFrontmatter holds the parsed frontmatter of a D-Mail file.
type DMailFrontmatter struct {
	SchemaVersion string            `yaml:"dmail-schema-version"`
	Name          string            `yaml:"name"`
	Kind          DMailKind         `yaml:"kind"`
	Description   string            `yaml:"description"`
	Action        string            `yaml:"action,omitempty"`
	Priority      int               `yaml:"priority,omitempty"`
	Targets       []string          `yaml:"targets,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
}

// ResolvedRoute is a concrete route with absolute paths for delivery.
type ResolvedRoute struct {
	Kind       DMailKind
	FromOutbox string   // absolute outbox directory path
	ToInboxes  []string // absolute inbox directory paths
}

// DeliveryResult holds the outcome of a single D-Mail delivery.
type DeliveryResult struct {
	SourcePath  string
	Kind        DMailKind
	DeliveredTo []string // inbox paths where the file was copied
}

// DMailKind is the type-safe representation of D-Mail message kinds.
type DMailKind string

// Canonical D-Mail kind constants.
const (
	KindSpecification    DMailKind = "specification"
	KindReport           DMailKind = "report"
	KindDesignFeedback   DMailKind = "design-feedback"
	KindImplFeedback     DMailKind = "implementation-feedback"
	KindConvergence      DMailKind = "convergence"
	KindCIResult         DMailKind = "ci-result"
	KindStallEscalation  DMailKind = "stall-escalation"
)

// validDMailKinds lists the allowed D-Mail kind values per schema v1.
var validDMailKinds = []DMailKind{KindSpecification, KindReport, KindDesignFeedback, KindImplFeedback, KindConvergence, KindCIResult, KindStallEscalation}

// IsValidDMailKind returns true if the given kind is in the canonical set.
func IsValidDMailKind(kind DMailKind) bool {
	return slices.Contains(validDMailKinds, kind)
}

// ErrDMailKindInvalid is returned when a D-Mail kind is not in the canonical set.
var ErrDMailKindInvalid = errors.New("dmail: invalid kind")

// ValidateKind checks that kind is one of the allowed D-Mail kinds.
func ValidateKind(kind DMailKind) error {
	if !IsValidDMailKind(kind) {
		return fmt.Errorf("invalid D-Mail kind %q: %w", kind, ErrDMailKindInvalid)
	}
	return nil
}

// ExtractDMailKind reads a D-Mail file's YAML frontmatter and returns the kind.
func ExtractDMailKind(data []byte) (DMailKind, error) {
	fm, err := ParseDMailFrontmatter(data)
	if err != nil {
		return "", err
	}
	if fm.SchemaVersion == "" {
		return "", errors.New("D-Mail missing required 'dmail-schema-version' field")
	}
	if fm.SchemaVersion != SupportedDMailSchemaVersion {
		return "", fmt.Errorf("unsupported dmail-schema-version %q: only \"1\" is supported", fm.SchemaVersion)
	}
	if fm.Kind == "" {
		return "", errors.New("D-Mail missing required 'kind' field")
	}
	if err := ValidateKind(fm.Kind); err != nil {
		return "", err
	}
	return fm.Kind, nil
}

// parseDMailFrontmatter extracts the YAML frontmatter from a D-Mail file.
// This is intentionally separate from ParseSkillFrontmatter because D-Mail
// and SKILL.md have different metadata structures (D-Mail metadata is
// map[string]string, while SKILL metadata has typed produces/consumes).
func ParseDMailFrontmatter(data []byte) (*DMailFrontmatter, error) {
	content := string(data)
	idx := findFrontmatterEnd(content)
	if idx < 0 {
		return nil, errors.New("no YAML frontmatter found: file must start with ---")
	}

	var fm DMailFrontmatter
	if err := yaml.Unmarshal([]byte(content[3:idx]), &fm); err != nil {
		return nil, err
	}
	return &fm, nil
}

// findFrontmatterEnd returns the byte offset of the closing "---" in content.
// Content must start with "---".
func findFrontmatterEnd(content string) int {
	if len(content) < 4 || content[:3] != "---" {
		return -1
	}
	rest := content[3:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\n' && i+3 < len(rest) && rest[i+1:i+4] == "---" {
			return 3 + i + 1
		}
	}
	return -1
}
