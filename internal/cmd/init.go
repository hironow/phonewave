package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
	"github.com/hironow/phonewave/internal/usecase"
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
			logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)

			cfgPath := configPath(cmd)
			if !force {
				if _, err := os.Stat(cfgPath); err == nil {
					return fmt.Errorf("already initialized: %s exists (use --force to overwrite)", cfgPath)
				}
			}

			initCmd := domain.InitCommand{
				RepoPaths:  args,
				ConfigPath: cfgPath,
			}
			result, err := usecase.RunInit(initCmd, &session.InitAdapter{})
			if err != nil {
				return err
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

			// Write .otel.env if --otel-backend is set
			otelBackend, _ := cmd.Flags().GetString("otel-backend")
			if otelBackend != "" {
				otelEntity, _ := cmd.Flags().GetString("otel-entity")
				otelProject, _ := cmd.Flags().GetString("otel-project")
				content, otelErr := platform.OtelEnvContent(otelBackend, otelEntity, otelProject)
				if otelErr != nil {
					return otelErr
				}
				stateDir := filepath.Join(filepath.Dir(cfgPath), domain.StateDir)
				if err := os.MkdirAll(stateDir, 0o755); err != nil {
					return fmt.Errorf("create state dir: %w", err)
				}
				otelPath := filepath.Join(stateDir, ".otel.env")
				if err := os.WriteFile(otelPath, []byte(content), 0o644); err != nil {
					return fmt.Errorf("write .otel.env: %w", err)
				}
				logger.OK("OTel backend configured: %s → %s", otelBackend, otelPath)
			}

			return nil
		},
	}

	cmd.Flags().Bool("force", false, "overwrite existing configuration")
	cmd.Flags().String("otel-backend", "", "OTel backend: jaeger, weave")
	cmd.Flags().String("otel-entity", "", "Weave entity/team (required for weave)")
	cmd.Flags().String("otel-project", "", "Weave project (required for weave)")

	return cmd
}
