package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave"
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
			cfgPath := configPath(cmd)
			cfg, err := phonewave.LoadConfig(cfgPath)
			if err != nil {
				phonewave.LogInfo("Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			orphans, err := phonewave.Remove(cfg, args[0])
			if err != nil {
				return err
			}

			if err := phonewave.WriteConfig(cfgPath, cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			absPath, _ := filepath.Abs(args[0])
			phonewave.LogOK("Removed %s", absPath)
			phonewave.LogOK("%d routes remaining", len(cfg.Routes))
			printOrphanWarnings(*orphans)

			return nil
		},
	}
}
