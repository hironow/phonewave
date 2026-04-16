package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/phonewave/internal/session"
	"github.com/spf13/cobra"
)

func newDeadLettersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dead-letters",
		Short: "Manage dead-lettered delivery items",
		Long:  "Inspect and manage delivery items that have exceeded the maximum retry count.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("no subcommand specified. Run 'phonewave dead-letters --help' for usage")
		},
	}

	cmd.AddCommand(newDeadLettersPurgeCmd())
	return cmd
}

func newDeadLettersPurgeCmd() *cobra.Command {
	var (
		execute bool
		yes     bool
	)

	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Purge dead-lettered delivery items",
		Long: `Remove delivery items that have permanently failed (exceeded max retry count).

By default, runs in dry-run mode showing how many items would be purged.
Pass --execute to actually delete dead-lettered items.`,
		Example: `  # Dry-run: show dead-letter count
  phonewave dead-letters purge

  # Delete dead-lettered items (with confirmation)
  phonewave dead-letters purge --execute

  # Delete without confirmation
  phonewave dead-letters purge --execute --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			stateDir := configBase(cmd)
			errW := cmd.ErrOrStderr()

			dbPath := filepath.Join(stateDir, ".run", "delivery.db")
			if _, err := os.Stat(dbPath); err != nil {
				fmt.Fprintln(errW, "No delivery database found.")
				return nil
			}

			store, err := session.NewSQLiteDeliveryStore(stateDir)
			if err != nil {
				return fmt.Errorf("open delivery store: %w", err)
			}
			defer store.Close()

			count, err := store.DeadLetterCount(cmd.Context())
			if err != nil {
				return fmt.Errorf("count dead letters: %w", err)
			}

			outputFmt, _ := cmd.Flags().GetString("output")
			jsonOut := outputFmt == "json"

			if count == 0 {
				if jsonOut {
					fmt.Fprintln(cmd.OutOrStdout(), `{"dead_letters":0,"purged":0}`)
				} else {
					fmt.Fprintln(errW, "No dead-lettered items.")
				}
				return nil
			}

			if !execute {
				if jsonOut {
					data, err := json.Marshal(map[string]int{"dead_letters": count, "purged": 0})
					if err != nil {
						return fmt.Errorf("marshal json: %w", err)
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				} else {
					fmt.Fprintf(errW, "%d dead-lettered item(s) would be purged (use --execute to delete)\n", count)
				}
				return nil
			}

			// --execute mode: confirm then purge
			if !yes {
				fmt.Fprintf(errW, "Delete %d dead-lettered item(s)? [y/N] ", count)
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

			purged, err := store.PurgeDeadLetters(cmd.Context())
			if err != nil {
				return fmt.Errorf("purge dead letters: %w", err)
			}
			if jsonOut {
				data, err := json.Marshal(map[string]int{"dead_letters": count, "purged": purged})
				if err != nil {
					return fmt.Errorf("marshal json: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintf(errW, "Purged %d dead-lettered item(s).\n", purged)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&execute, "execute", false, "Execute purge (default: dry-run)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")

	return cmd
}
