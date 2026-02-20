package phonewave

import (
	"os"
	"path/filepath"
)

// StateDir is the name of the phonewave state directory.
const StateDir = ".phonewave"

// ConfigFile is the default name of the phonewave configuration file.
const ConfigFile = "phonewave.yaml"

// EnsureStateDir creates the .phonewave/ state directory structure.
func EnsureStateDir(base string) error {
	dirs := []string{
		filepath.Join(base, StateDir),
		filepath.Join(base, StateDir, "errors"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}
