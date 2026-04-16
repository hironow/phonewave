package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <repo-path>",
		Short: "Add a new repository to the ecosystem",
		Long:  "Add a new repository to the phonewave ecosystem, scan its endpoints, and update the routing table.",
		Args:  cobra.ExactArgs(1),
		Example: `  phonewave add ./new-repo
  phonewave add /absolute/path/to/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose := mustBool(cmd, "verbose")
			logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)

			cfgPath := configPath(cmd)
			cfg, err := session.LoadConfig(cfgPath)
			if err != nil {
				logger.Info("Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			result, err := session.Add(cmd.Context(), cfg, args[0])
			if err != nil {
				return err
			}

			if err := session.WriteConfig(cfgPath, cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			result.RouteCount = len(cfg.Routes)

			absPath, _ := filepath.Abs(args[0])
			logger.OK("Added %s", absPath)
			logger.OK("%d routes total", result.RouteCount)
			printOrphanWarnings(logger, result.Orphans)
			for _, w := range result.Warnings {
				logger.Warn("%s", w)
			}

			return nil
		},
	}
}
