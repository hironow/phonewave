package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [path]",
		Short: "Show daemon and delivery status",
		Long: `Show daemon state, uptime, watched directories, route count,
error queue, and 24h delivery statistics.

Output goes to stdout by default (human-readable text).
Use -o json for machine-readable JSON output to stdout.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  # Show status for default config location
  phonewave status

  # Show status for a specific project
  phonewave status /path/to/project

  # JSON output for scripting
  phonewave status -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveBaseDir(cmd, args)
			if err != nil {
				return err
			}
			cfgPath := filepath.Join(base, domain.StateDir, domain.ConfigFile)
			stateDir := filepath.Join(base, domain.StateDir)

			cfg, err := session.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w (run 'phonewave init' first)", err)
			}

			status := session.Status(cfg, stateDir)

			outputFmt, _ := cmd.Flags().GetString("output")
			if outputFmt == "json" {
				data, jsonErr := json.Marshal(status)
				if jsonErr != nil {
					return fmt.Errorf("marshal status: %w", jsonErr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// Text output to stdout (human-readable, per S0027)
			fmt.Fprint(cmd.OutOrStdout(), status.FormatText())
			return nil
		},
	}
}
