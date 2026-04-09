package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
	"github.com/spf13/cobra"
)

// Version, Commit, and Date are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "dev"
	Date    = "dev"
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
		SilenceErrors: true, // nosemgrep: cobra-silence-errors-without-output — main.go prints err to os.Stderr when ExecuteContext fails [permanent]
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			noColor, _ := cmd.Flags().GetBool("no-color")
			if noColor {
				os.Setenv("NO_COLOR", "1")
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			out := cmd.ErrOrStderr()
			quiet, _ := cmd.Flags().GetBool("quiet")
			if quiet {
				out = io.Discard
			}
			logger := platform.NewLogger(out, verbose)
			outputFmt, _ := cmd.Flags().GetString("output")
			if outputFmt != "json" {
				logger.Header("phonewave", Version)
				logger.Section(cmd.Name())
			}
			if migErr := session.MigrateConfigIfNeeded(projectRoot(cmd)); migErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: config migration: %v\n", migErr)
			}
			applyOtelEnv(configBase(cmd))
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
				shutdownMeterFn(context.Background())
			}
			if shutdownTracerFn != nil {
				shutdownTracerFn(context.Background())
			}
		})
	})

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Log all delivery events to stderr")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output (respects NO_COLOR env)")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress all stderr output")
	rootCmd.PersistentFlags().StringP("config", "c", filepath.Join(".", domain.StateDir, domain.ConfigFile), "Path to phonewave config file")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text, json")
	rootCmd.PersistentFlags().Bool("linear", false, "Use Linear MCP for issue tracking (default: wave-centric mode)")

	rootCmd.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newSyncCmd(),
		newDoctorCmd(),
		newRunCmd(),
		newStatusCmd(),
		newMetricsCmd(),
		newCleanCmd(),
		newArchivePruneCmd(),
		newDeadLettersCmd(),
		newVersionCmd(),
		newUpdateCmd(),
	)

	return rootCmd
}
