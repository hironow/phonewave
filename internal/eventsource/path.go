package eventsource

import "path/filepath"

// EventsDir returns the path to the events directory under stateDir.
func EventsDir(stateDir string) string {
	return filepath.Join(stateDir, "events")
}
