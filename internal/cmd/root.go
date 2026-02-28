package cmd

import (
	"path/filepath"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

// Build info variables, set via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func init() {
	cobra.EnableTraverseRunHooks = true
}

// NewRootCommand creates the root cobra command for phonewave.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "phonewave",
		Short:         "D-Mail courier daemon",
		Long:          "Phonewave routes D-Mails between AI agent tool repositories via file-based message passing.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Log all delivery events to stderr")
	rootCmd.PersistentFlags().StringP("config", "c", filepath.Join(".", phonewave.ConfigFile), "Path to phonewave config file")

	rootCmd.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newSyncCmd(),
		newDoctorCmd(),
		newRunCmd(),
		newStatusCmd(),
		newCleanCmd(),
		newVersionCmd(),
		newUpdateCmd(),
	)

	return rootCmd
}
