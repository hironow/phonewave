package cmd

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

// Build info variables, set via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// shutdownTracer holds the OTel tracer shutdown function registered by
// PersistentPreRunE. cobra.OnFinalize calls it after Execute completes.
var (
	shutdownTracer func(context.Context) error
	finalizerOnce  sync.Once
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			shutdownTracer = initTracer("phonewave", Version)
			return nil
		},
	}

	finalizerOnce.Do(func() {
		cobra.OnFinalize(func() {
			if shutdownTracer != nil {
				shutdownTracer(context.Background())
			}
		})
	})

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
		newVersionCmd(),
		newUpdateCmd(),
	)

	return rootCmd
}
