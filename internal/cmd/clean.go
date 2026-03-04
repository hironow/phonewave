package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove runtime state from .phonewave/",
		Long: `Remove runtime state files from the .phonewave/ directory.

Removes: delivery.log, .run/, watch.pid, watch.started, events/
Preserves: phonewave.yaml (config) and .phonewave/.gitignore`,
		Example: `  phonewave clean
  phonewave clean --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			base := configBase(cmd)
			stateDir := filepath.Join(base, domain.StateDir)

			info, err := os.Stat(stateDir)
			if err != nil || !info.IsDir() {
				fmt.Fprintf(cmd.ErrOrStderr(), "Nothing to clean at %s\n", stateDir)
				return nil
			}

			if daemonRunning(stateDir) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: daemon appears to be running. Stop it first for a clean reset.\n")
			}

			targets := collectCleanTargets(stateDir)
			if len(targets) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Nothing to clean.\n")
				return nil
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "The following will be deleted:\n")
				for _, t := range targets {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", t)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "\nDelete? [y/N]: ")
				var answer string
				fmt.Fscanln(cmd.InOrStdin(), &answer)
				if answer != "y" && answer != "Y" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Aborted.\n")
					return nil
				}
			}

			removed := 0
			for _, t := range targets {
				if err := os.RemoveAll(t); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  failed to remove %s: %v\n", t, err)
				} else {
					removed++
				}
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Cleaned %d item(s) from %s\n", removed, stateDir)
			return nil
		},
	}
	cmd.Flags().Bool("yes", false, "skip confirmation prompt")
	return cmd
}

// collectCleanTargets returns paths to state files/directories that should be
// cleaned. The config file (phonewave.yaml) and .gitignore are excluded.
func collectCleanTargets(stateDir string) []string {
	candidates := []string{
		"delivery.log",
		".run",
		"watch.pid",
		"watch.started",
		"events",
	}
	var targets []string
	for _, name := range candidates {
		p := filepath.Join(stateDir, name)
		if _, err := os.Stat(p); err == nil {
			targets = append(targets, p)
		}
	}
	return targets
}

// daemonRunning checks if a daemon process is alive by reading watch.pid and
// sending signal 0.
func daemonRunning(stateDir string) bool {
	data, err := os.ReadFile(filepath.Join(stateDir, "watch.pid"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil || pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
