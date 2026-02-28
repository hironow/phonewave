package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <repo-path> [repo-path...]",
		Short: "Scan repositories, discover tools, generate routing table",
		Long:  "Scan one or more repositories for tool endpoints, parse SKILL.md manifests, derive a routing table, and generate phonewave.yaml.",
		Args:  cobra.MinimumNArgs(1),
		Example: `  phonewave init ./sightjack-repo ./paintress-repo ./amadeus-repo
  phonewave init /absolute/path/to/repo
  phonewave init --force ./repo  # overwrite existing config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			force, _ := cmd.Flags().GetBool("force")
			logger := phonewave.NewLogger(cmd.ErrOrStderr(), verbose)

			cfgPath := configPath(cmd)
			if !force {
				if _, err := os.Stat(cfgPath); err == nil {
					return fmt.Errorf("already initialized: %s exists (use --force to overwrite)", cfgPath)
				}
			}

			result, err := phonewave.Init(args)
			if err != nil {
				return err
			}

			if err := phonewave.WriteConfig(cfgPath, result.Config); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			if err := phonewave.EnsureStateDir(configBase(cmd)); err != nil {
				return fmt.Errorf("create state dir: %w", err)
			}

			logger.OK("Scanned %d repositories", result.RepoCount)
			for _, repo := range result.Config.Repositories {
				for _, ep := range repo.Endpoints {
					logger.OK("  %s/%s  produces=%v consumes=%v", filepath.Base(repo.Path), ep.Dir, ep.Produces, ep.Consumes)
				}
			}
			logger.OK("Derived %d routes", len(result.Config.Routes))
			for _, r := range result.Config.Routes {
				logger.Info("  %s: %s → %v", r.Kind, r.From, r.To)
			}

			printOrphanWarnings(logger, result.Orphans)

			logger.OK("Config written to %s", cfgPath)
			return nil
		},
	}

	cmd.Flags().Bool("force", false, "overwrite existing configuration")

	return cmd
}
