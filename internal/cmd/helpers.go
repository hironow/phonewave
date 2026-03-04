package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/spf13/cobra"
)

func configPath(cmd *cobra.Command) string {
	p, _ := cmd.Flags().GetString("config")
	return p
}

// configBase returns the directory containing the config file.
// State directory and EnsureStateDir should use this as their base
// so that daemon state (PID, error queue, delivery.log) lives alongside
// the config rather than being tied to the current working directory.
func configBase(cmd *cobra.Command) string {
	return filepath.Dir(configPath(cmd))
}

// resolveBaseDir returns the base directory for phonewave state.
// If args[0] is provided, uses that directory.
// Otherwise, falls back to the --config flag's parent directory.
func resolveBaseDir(cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return "", fmt.Errorf("resolve path: %w", err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("path not found: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("not a directory: %s", abs)
		}
		return abs, nil
	}
	return filepath.Abs(configBase(cmd))
}

func printOrphanWarnings(logger domain.Logger, orphans domain.OrphanReport) {
	for _, kind := range orphans.UnconsumedKinds {
		logger.Warn("Orphaned: kind=%q is produced but not consumed", kind)
	}
	for _, kind := range orphans.UnproducedKinds {
		logger.Warn("Orphaned: kind=%q is consumed but not produced", kind)
	}
}
