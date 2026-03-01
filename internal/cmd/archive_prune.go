package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave"
	"github.com/hironow/phonewave/internal/session"
	"github.com/spf13/cobra"
)

func newArchivePruneCmd() *cobra.Command {
	var (
		execute bool
		days    int
	)

	cmd := &cobra.Command{
		Use:   "archive-prune",
		Short: "Prune expired event files",
		Long: `Prune expired event files from the events directory.

Lists event files older than the retention threshold.
By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.`,
		Example: `  # Dry-run: list expired files (default 30 days)
  phonewave archive-prune

  # Delete expired files
  phonewave archive-prune --execute

  # Custom retention period
  phonewave archive-prune --days 7 --execute`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			base := configBase(cmd)
			stateDir := filepath.Join(base, phonewave.StateDir)
			errW := cmd.ErrOrStderr()

			files, err := session.ListExpiredEventFiles(stateDir, days)
			if err != nil {
				return fmt.Errorf("failed to list expired events: %w", err)
			}

			if len(files) == 0 {
				fmt.Fprintf(errW, "No expired event files (threshold: %d days).\n", days)
				return nil
			}

			fmt.Fprintln(errW, "Expired event files:")
			for _, f := range files {
				fmt.Fprintln(errW, "  "+f)
			}
			fmt.Fprintf(errW, "%d event file(s) older than %d days.\n", len(files), days)

			if !execute {
				fmt.Fprintln(errW, "(dry-run — pass --execute to delete)")
				return nil
			}

			deleted, delErr := session.PruneEventFiles(stateDir, files)
			if delErr != nil {
				return fmt.Errorf("event prune failed: %w", delErr)
			}
			fmt.Fprintf(errW, "Pruned %d event file(s).\n", len(deleted))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&execute, "execute", "x", false, "Execute pruning (default: dry-run)")
	cmd.Flags().IntVarP(&days, "days", "d", 30, "Retention days")

	return cmd
}
