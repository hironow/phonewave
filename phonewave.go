package phonewave

import (
	"os"
	"path/filepath"
)

// StateDir is the name of the phonewave state directory.
const StateDir = ".phonewave"

// ConfigFile is the default name of the phonewave configuration file.
const ConfigFile = "phonewave.yaml"

// EnsureStateDir creates the .phonewave/ state directory structure and
// writes a .gitignore so runtime state is not accidentally committed.
func EnsureStateDir(base string) error {
	stateDir := filepath.Join(base, StateDir)
	dirs := []string{
		stateDir,
		filepath.Join(stateDir, "errors"),
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
