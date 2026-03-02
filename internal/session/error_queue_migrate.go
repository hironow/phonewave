package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	phonewave "github.com/hironow/phonewave"
)

// MigrateFileErrorQueue migrates legacy .err sidecar files from
// {stateDir}/errors/ into the SQLiteErrorQueueStore. This is idempotent:
// Enqueue uses INSERT OR IGNORE, so re-running is safe.
// Successfully migrated files (both data file and .err sidecar) are removed.
func MigrateFileErrorQueue(stateDir string, store phonewave.ErrorQueueStore, logger *phonewave.Logger) (int, error) {
	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("migrate: read errors dir: %w", err)
	}

	migrated := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".err") {
			continue
		}

		sidecarPath := filepath.Join(errorsDir, entry.Name())
		meta, err := LoadErrorMetadata(sidecarPath)
		if err != nil {
			logger.Warn("migrate: load metadata %s: %v", sidecarPath, err)
			continue
		}

		// The data file is the sidecar without the .err extension
		dataPath := strings.TrimSuffix(sidecarPath, ".err")
		data, err := os.ReadFile(dataPath)
		if err != nil {
			logger.Warn("migrate: read data %s: %v", dataPath, err)
			continue
		}

		name := filepath.Base(dataPath)
		if err := store.Enqueue(name, data, *meta); err != nil {
			logger.Warn("migrate: enqueue %s: %v", name, err)
			continue
		}

		// Successfully migrated — remove both files
		os.Remove(dataPath)
		os.Remove(sidecarPath)
		migrated++
	}

	return migrated, nil
}
