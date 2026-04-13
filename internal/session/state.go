package session

import (
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
)

// phonewaveGitignoreEntries lists paths that must be gitignored in .phonewave/.
var phonewaveGitignoreEntries = []string{
	"watch.pid",
	"watch.started",
	"provider-state.json",
	"delivery.log",
	"events/",
	".run/",
	".otel.env",
	"!config.yaml",
}

// EnsurePhonewaveStateDir creates the .phonewave/ state directory structure and
// writes a .gitignore so runtime state is not accidentally committed.
// Delegates to the shared EnsureStateDir helper for core directories.
func EnsurePhonewaveStateDir(base string) error {
	stateDir := filepath.Join(base, domain.StateDir)

	// Core directories (no mail dirs, no skills — phonewave is a courier)
	if _, err := EnsureStateDir(stateDir); err != nil {
		return err
	}

	// Gitignore (append-only, phonewave-specific entries)
	return EnsureGitignoreEntries(filepath.Join(stateDir, ".gitignore"), phonewaveGitignoreEntries)
}
