package session

import (
	"os"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
)

// EnsureStateDir creates the .phonewave/ state directory structure and
// writes a .gitignore so runtime state is not accidentally committed.
func EnsureStateDir(base string) error {
	stateDir := filepath.Join(base, domain.StateDir)
	dirs := []string{
		stateDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	gitignorePath := filepath.Join(stateDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitignorePath, []byte("# phonewave runtime state — do not commit\n*\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}
