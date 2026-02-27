package session

import (
	"os"
	"path/filepath"

	phonewave "github.com/hironow/phonewave"
)

// EnsureStateDir creates the .phonewave/ state directory structure.
func EnsureStateDir(base string) error {
	dirs := []string{
		filepath.Join(base, phonewave.StateDir),
		filepath.Join(base, phonewave.StateDir, ".run"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}
