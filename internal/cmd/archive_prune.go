package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase"
	"github.com/spf13/cobra"
)

func newArchivePruneCmd() *cobra.Command {
	var (
		execute bool
		days    int
	)

	cmd := &cobra.Command{
		Use:   "archive-prune [path]",
		Short: "Prune expired event files",
		Long: `Prune expired event files from the events directory.

Lists event files older than the retention threshold.
By default, runs in dry-run mode showing what would be deleted.
Pass --execute to actually remove the files.`,
		Example: `  # Dry-run: list expired files (default 30 days)
  phonewave archive-prune

  # Delete expired files
  phonewave archive-prune --execute

  # Specific project directory
  phonewave archive-prune /path/to/project --execute

  # JSON output for scripting
  phonewave archive-prune -o json

  # Custom retention period
  phonewave archive-prune --days 7 --execute`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if execute && cmd.Flags().Changed("dry-run") {
				return fmt.Errorf("--execute and --dry-run are mutually exclusive")
			}
			base, err := resolveBaseDir(cmd, args)
			if err != nil {
				return err
			}
			stateDir := filepath.Join(base, domain.StateDir)
			outputFmt, _ := cmd.Flags().GetString("output")
			errW := cmd.ErrOrStderr()

			files, err := usecase.ListExpiredEventFiles(stateDir, days)
			if err != nil {
				return fmt.Errorf("failed to list expired events: %w", err)
			}

			if outputFmt == "json" {
				out := struct {
					Candidates int      `json:"candidates"`
					Deleted    int      `json:"deleted"`
					Files      []string `json:"files"`
				}{
					Candidates: len(files),
					Files:      files,
				}
				if execute && len(files) > 0 {
					deleted, delErr := usecase.PruneEventFiles(stateDir, files)
					if delErr != nil {
						return fmt.Errorf("event prune failed: %w", delErr)
					}
					out.Deleted = len(deleted)
				}
				data, jsonErr := json.Marshal(out)
				if jsonErr != nil {
					return jsonErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// text output
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

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				fmt.Fprintf(errW, "\nDelete these %d file(s)? [y/N] ", len(files))
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if !scanner.Scan() {
					if scanErr := scanner.Err(); scanErr != nil {
						return fmt.Errorf("read confirmation: %w", scanErr)
					}
					fmt.Fprintln(errW, "Cancelled.")
					return nil
				}
				answer := strings.TrimSpace(scanner.Text())
				if answer != "y" && answer != "Y" {
					fmt.Fprintln(errW, "Cancelled.")
					return nil
				}
			}

			deleted, delErr := usecase.PruneEventFiles(stateDir, files)
			if delErr != nil {
				return fmt.Errorf("event prune failed: %w", delErr)
			}
			fmt.Fprintf(errW, "Pruned %d event file(s).\n", len(deleted))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&execute, "execute", "x", false, "Execute pruning (default: dry-run)")
	cmd.Flags().IntVarP(&days, "days", "d", 30, "Retention days")
	cmd.Flags().BoolP("dry-run", "n", false, "Dry-run mode (default behavior, explicit for scripting)")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}
