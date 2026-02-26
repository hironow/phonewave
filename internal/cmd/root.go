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

type loggerKeyType struct{}

var loggerKey loggerKeyType

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
			verbose, _ := cmd.Flags().GetBool("verbose")
			logger := phonewave.NewLogger(cmd.ErrOrStderr(), verbose)
			ctx := context.WithValue(cmd.Context(), loggerKey, logger)
			shutdownTracer = initTracer("phonewave", Version)
			cmd.SetContext(ctx)
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

// loggerFrom extracts the *phonewave.Logger from the cobra command context.
// Falls back to a stderr logger if PersistentPreRunE was not executed (e.g., in tests).
func loggerFrom(cmd *cobra.Command) *phonewave.Logger {
	if l, ok := cmd.Context().Value(loggerKey).(*phonewave.Logger); ok {
		return l
	}
	return phonewave.NewLogger(cmd.ErrOrStderr(), false)
}
