package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave"
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
			configPath := filepath.Join(".", phonewave.ConfigFile)
			cfg, err := phonewave.LoadConfig(configPath)
			if err != nil {
				phonewave.LogInfo("Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			orphans, err := phonewave.Add(cfg, args[0])
			if err != nil {
				return err
			}

			if err := phonewave.WriteConfig(configPath, cfg); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			absPath, _ := filepath.Abs(args[0])
			phonewave.LogOK("Added %s", absPath)
			phonewave.LogOK("%d routes total", len(cfg.Routes))
			printOrphanWarnings(*orphans)

			return nil
		},
	}
}
