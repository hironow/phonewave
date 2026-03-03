package cmd

import (
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <repo-path>",
		Short: "Remove a repository from the ecosystem",
		Long:  "Remove a repository from the phonewave ecosystem and update the routing table.",
		Args:  cobra.ExactArgs(1),
		Example: `  phonewave remove ./old-repo
  phonewave remove /absolute/path/to/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			logger := domain.NewLogger(cmd.ErrOrStderr(), verbose)

			cfgPath := configPath(cmd)
			result, err := usecase.RemoveRepository(cfgPath, args[0], logger)
			if err != nil {
				logger.Info("Run 'phonewave init' first")
				return err
			}

			absPath, _ := filepath.Abs(args[0])
			logger.OK("Removed %s", absPath)
			logger.OK("%d routes remaining", result.RouteCount)
			printOrphanWarnings(logger, result.Orphans)

			return nil
		},
	}
}
