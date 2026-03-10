package session

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
)

// phonewaveGitignoreEntries lists paths that must be gitignored in .phonewave/.
var phonewaveGitignoreEntries = []string{
	"watch.pid",
	"watch.started",
	"delivery.log",
	"events/",
	".run/",
	".otel.env",
	"!config.yaml",
}

// EnsureStateDir creates the .phonewave/ state directory structure and
// writes a .gitignore so runtime state is not accidentally committed.
// Uses append-only pattern: existing user entries are preserved.
func EnsureStateDir(base string) error {
	stateDir := filepath.Join(base, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "insights"), 0755); err != nil {
		return err
	}
	return ensureGitignoreEntries(filepath.Join(stateDir, ".gitignore"), phonewaveGitignoreEntries)
}

// ensureGitignoreEntries reads an existing .gitignore (if any) and appends
// any missing entries. Creates the file with all entries if it does not exist.
func ensureGitignoreEntries(path string, required []string) error {
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	var missing []string
	for _, entry := range required {
		if !strings.Contains(existing, entry) {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if existing == "" {
		// New file: write header + all entries
		var buf strings.Builder
		buf.WriteString("# phonewave runtime state — do not commit\n")
		for _, entry := range required {
			buf.WriteString(entry)
			buf.WriteByte('\n')
		}
		return os.WriteFile(path, []byte(buf.String()), 0o644)
	}

	// Existing file: append missing entries
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	var buf strings.Builder
	buf.WriteString(existing)
	for _, entry := range missing {
		buf.WriteString(entry)
		buf.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}
