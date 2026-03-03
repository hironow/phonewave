package cmd

import (
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase"
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
			verbose, _ := cmd.Flags().GetBool("verbose")
			logger := domain.NewLogger(cmd.ErrOrStderr(), verbose)

			cfgPath := configPath(cmd)
			result, err := usecase.AddRepository(cfgPath, args[0], logger)
			if err != nil {
				logger.Info("Run 'phonewave init' first")
				return err
			}

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
