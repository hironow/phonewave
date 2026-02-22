package cmd

import (
	"fmt"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Short:   "Re-scan all repositories, reconcile routing table",
		Long:    "Re-scan all repositories in the ecosystem, detect endpoint changes, and reconcile the routing table.",
		Args:    cobra.NoArgs,
		Example: `  phonewave sync`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			logger := phonewave.NewLogger(cmd.ErrOrStderr(), verbose)

			cfgPath := configPath(cmd)
			cfg, err := phonewave.LoadConfig(cfgPath)
			if err != nil {
				logger.Info("Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			report, err := phonewave.Sync(cfg)
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}

			if err := phonewave.WriteConfig(cfgPath, cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			logger.OK("Synced %d repositories, %d routes", report.RepoCount, report.TotalRoutes)

			for _, d := range report.EndpointChanges {
				switch d.Change {
				case "added":
					logger.OK("  + endpoint %s/%s", d.Repo, d.Dir)
				case "removed":
					logger.Warn("  - endpoint %s/%s", d.Repo, d.Dir)
				case "changed":
					logger.Info("  ~ endpoint %s/%s", d.Repo, d.Dir)
				}
			}
			for _, d := range report.RouteChanges {
				switch d.Change {
				case "added":
					logger.OK("  + route %s from %s", d.Kind, d.From)
				case "removed":
					logger.Warn("  - route %s from %s", d.Kind, d.From)
				}
			}

			printOrphanWarnings(logger, report.Orphans)
			for _, w := range report.Warnings {
				logger.Warn("%s", w)
			}

			return nil
		},
	}
}
