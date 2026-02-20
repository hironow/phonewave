package cmd

import (
	"context"
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

// shutdownTracer holds the OTel tracer shutdown function.
// Set in PersistentPreRunE, called in OnFinalize.
var shutdownTracer func(context.Context) error

// NewRootCommand creates the root cobra command for phonewave.
func NewRootCommand() *cobra.Command {
	cobra.EnableTraverseRunHooks = true

	rootCmd := &cobra.Command{
		Use:   "phonewave",
		Short: "D-Mail courier daemon",
		Long:  "Phonewave routes D-Mails between AI agent tool repositories via file-based message passing.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			shutdownTracer = phonewave.InitTracer("phonewave", Version)
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cobra.OnFinalize(func() {
		if shutdownTracer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5_000_000_000) // 5s
			defer cancel()
			if err := shutdownTracer(ctx); err != nil {
				phonewave.LogWarn("tracer shutdown: %v", err)
			}
		}
	})

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Log all delivery events to stderr")
	rootCmd.PersistentFlags().String("config", filepath.Join(".", phonewave.ConfigFile), "Path to phonewave config file")

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
