package phonewave

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

// DMailFrontmatter holds the parsed frontmatter of a D-Mail file.
type DMailFrontmatter struct {
	SchemaVersion string `yaml:"dmail-schema-version"`
	Name          string `yaml:"name"`
	Kind          string `yaml:"kind"`
	Description   string `yaml:"description"`
}

// ResolvedRoute is a concrete route with absolute paths for delivery.
type ResolvedRoute struct {
	Kind       string
	FromOutbox string   // absolute outbox directory path
	ToInboxes  []string // absolute inbox directory paths
}

// DeliveryResult holds the outcome of a single D-Mail delivery.
type DeliveryResult struct {
	SourcePath  string
	Kind        string
	DeliveredTo []string // inbox paths where the file was copied
}

// validKinds lists the allowed D-Mail kind values per schema v1.
var validKinds = []string{"specification", "report", "feedback", "convergence"}

// ValidKinds returns a copy of the allowed D-Mail kind values.
func ValidKinds() []string {
	return append([]string(nil), validKinds...)
}

// ValidateKind checks that kind is one of the allowed D-Mail kinds.
func ValidateKind(kind string) error {
	if !slices.Contains(validKinds, kind) {
		return fmt.Errorf("invalid D-Mail kind %q: must be one of %v", kind, validKinds)
	}
	return nil
}

// ExtractDMailKind reads a D-Mail file's YAML frontmatter and returns the kind.
func ExtractDMailKind(data []byte) (string, error) {
	fm, err := parseDMailFrontmatter(data)
	if err != nil {
		return "", err
	}
	if fm.SchemaVersion == "" {
		return "", errors.New("D-Mail missing required 'dmail-schema-version' field")
	}
	if fm.SchemaVersion != "1" {
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
func parseDMailFrontmatter(data []byte) (*DMailFrontmatter, error) {
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

// Deliver reads a D-Mail file and delivers it to all matching inboxes.
func Deliver(ctx context.Context, dmailPath string, routes []ResolvedRoute) (*DeliveryResult, error) {
	data, err := os.ReadFile(dmailPath)
	if err != nil {
		return nil, fmt.Errorf("read D-Mail: %w", err)
	}
	return DeliverData(ctx, dmailPath, data, routes)
}

// DeliverData processes pre-read D-Mail data: routes by kind,
// copies to all target inboxes atomically, then removes the source.
func DeliverData(ctx context.Context, dmailPath string, data []byte, routes []ResolvedRoute) (*DeliveryResult, error) {
	kind, err := ExtractDMailKind(data)
	if err != nil {
		return nil, fmt.Errorf("parse D-Mail %s: %w", dmailPath, err)
	}

	ctx, span := tracer.Start(ctx, "delivery.deliver",
		trace.WithAttributes(
			attribute.String("dmail.path", dmailPath),
			attribute.String("dmail.kind", kind),
		),
	)
	defer span.End()

	// Find matching route
	sourceDir := filepath.Dir(dmailPath)
	var matchedRoute *ResolvedRoute
	for i := range routes {
		if routes[i].Kind == kind && routes[i].FromOutbox == sourceDir {
			matchedRoute = &routes[i]
			break
		}
	}
	if matchedRoute == nil {
		err := fmt.Errorf("no route for kind=%q from %s", kind, sourceDir)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	fileName := filepath.Base(dmailPath)
	result := &DeliveryResult{
		SourcePath: dmailPath,
		Kind:       kind,
	}

	// Copy to all target inboxes (atomic: write temp → rename).
	// On failure, roll back already-written files to prevent duplicates on retry.
	for _, inbox := range matchedRoute.ToInboxes {
		targetPath := filepath.Join(inbox, fileName)
		if err := atomicWrite(targetPath, data); err != nil {
			// Roll back all previously written inbox files
			for _, written := range result.DeliveredTo {
				os.Remove(written)
			}
			result.DeliveredTo = nil
			deliverErr := fmt.Errorf("deliver to %s: %w", inbox, err)
			span.RecordError(deliverErr)
			span.SetStatus(codes.Error, deliverErr.Error())
			return result, deliverErr
		}
		result.DeliveredTo = append(result.DeliveredTo, targetPath)
	}

	// Remove source only after all deliveries succeed (at-least-once).
	// Ignore ErrNotExist: the source may already have been cleaned up
	// (e.g. retry delivery from error queue).
	if err := os.Remove(dmailPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		removeErr := fmt.Errorf("remove source %s: %w", dmailPath, err)
		span.RecordError(removeErr)
		span.SetStatus(codes.Error, removeErr.Error())
		return result, removeErr
	}

	span.SetAttributes(attribute.Int("inbox.count", len(result.DeliveredTo)))
	return result, nil
}

// atomicWrite writes data to a temporary file in the same directory,
// then renames it to the target path (atomic on same filesystem).
func atomicWrite(targetPath string, data []byte) error {
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, ".phonewave-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, targetPath)
}
