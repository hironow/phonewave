package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hironow/phonewave/internal/domain"
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
	shutdownTracerFn func(context.Context) error
	shutdownMeterFn  func(context.Context) error
	finalizerOnce    sync.Once
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
			shutdownTracerFn = initTracer("phonewave", Version)
			shutdownMeterFn = initMeter("phonewave", Version)
			spanCtx := startRootSpan(cmd.Context(), cmd.Name())
			cmd.SetContext(spanCtx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("no subcommand specified. Run 'phonewave help' for usage")
		},
	}

	finalizerOnce.Do(func() {
		cobra.OnFinalize(func() {
			endRootSpan()
			if shutdownMeterFn != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := shutdownMeterFn(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "meter shutdown: %v\n", err)
				}
			}
			if shutdownTracerFn != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := shutdownTracerFn(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "tracer shutdown: %v\n", err)
				}
			}
		})
	})

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Log all delivery events to stderr")
	rootCmd.PersistentFlags().StringP("config", "c", filepath.Join(".", domain.ConfigFile), "Path to phonewave config file")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text, json")

	rootCmd.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newSyncCmd(),
		newDoctorCmd(),
		newRunCmd(),
		newStatusCmd(),
		newCleanCmd(),
		newArchivePruneCmd(),
		newVersionCmd(),
		newUpdateCmd(),
	)

	return rootCmd
}
