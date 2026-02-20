package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <repo-path> [repo-path...]",
		Short: "Scan repositories, discover tools, generate routing table",
		Long:  "Scan one or more repositories for tool endpoints, parse SKILL.md manifests, derive a routing table, and generate phonewave.yaml.",
		Args:  cobra.MinimumNArgs(1),
		Example: `  phonewave init ./sightjack-repo ./paintress-repo ./amadeus-repo
  phonewave init /absolute/path/to/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := phonewave.Init(args)
			if err != nil {
				return err
			}

			cfgPath := configPath(cmd)
			if err := phonewave.WriteConfig(cfgPath, result.Config); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			if err := phonewave.EnsureStateDir("."); err != nil {
				return fmt.Errorf("create state dir: %w", err)
			}

			phonewave.LogOK("Scanned %d repositories", result.RepoCount)
			for _, repo := range result.Config.Repositories {
				for _, ep := range repo.Endpoints {
					phonewave.LogOK("  %s/%s  produces=%v consumes=%v", filepath.Base(repo.Path), ep.Dir, ep.Produces, ep.Consumes)
				}
			}
			phonewave.LogOK("Derived %d routes", len(result.Config.Routes))
			for _, r := range result.Config.Routes {
				phonewave.LogInfo("  %s: %s → %v", r.Kind, r.From, r.To)
			}

			printOrphanWarnings(result.Orphans)

			phonewave.LogOK("Config written to %s", cfgPath)
			return nil
		},
	}
}
