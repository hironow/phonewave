package domain

import (
	"path/filepath"
	"time"
)

// StateDir is the name of the phonewave state directory.
const StateDir = ".phonewave"

// SkillsRefVenvName is the directory name for the skills-ref Python venv.
const SkillsRefVenvName = "phonewave-skills-ref-venv"

// ConfigFile is the default name of the phonewave configuration file.
const ConfigFile = "config.yaml"

// LegacyConfigFile is the old config filename used before the state-dir migration.
const LegacyConfigFile = "phonewave.yaml"

// DefaultConfigPath returns the standard config file path for a given project root.
func DefaultConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, StateDir, ConfigFile)
}

// ResolvedStateFile is the filename for the local resolved state (gitignored).
const ResolvedStateFile = "resolved.yaml"

// ResolvedState holds machine-local derived data: routes and sync metadata.
// Stored in .phonewave/.run/resolved.yaml, separate from the git-tracked manifest.
type ResolvedState struct {
	LastSynced time.Time     `yaml:"last_synced"`
	Routes     []RouteConfig `yaml:"routes"`
}
