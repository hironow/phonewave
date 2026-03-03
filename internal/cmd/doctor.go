package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/usecase"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "doctor",
		Short:   "Verify ecosystem health",
		Long:    "Check ecosystem health: verify paths, endpoints, SKILL.md spec compliance, PID conflicts, and daemon status.",
		Args:    cobra.NoArgs,
		Example: `  phonewave doctor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			outputFmt, _ := cmd.Flags().GetString("output")
			jsonOut := outputFmt == "json"
			logger := domain.NewLogger(cmd.ErrOrStderr(), verbose)

			cfgPath := configPath(cmd)
			stateDir := filepath.Join(configBase(cmd), domain.StateDir)
			report, err := usecase.RunDoctor(cfgPath, stateDir)
			if err != nil {
				logger.Info("Run 'phonewave init' first")
				return err
			}

			if jsonOut {
				data, err := usecase.FormatDoctorJSON(report)
				if err != nil {
					return fmt.Errorf("format JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				if !report.Healthy {
					return fmt.Errorf("ecosystem has issues")
				}
				return nil
			}

			for _, issue := range report.Issues {
				switch issue.Severity {
				case "ok":
					logger.OK("%s  %s", issue.Endpoint, issue.Message)
				case "fixed":
					logger.Warn("%s  %s", issue.Endpoint, issue.Message)
				case "warn":
					logger.Warn("%s  %s", issue.Endpoint, issue.Message)
				case "error":
					logger.Error("%s  %s", issue.Endpoint, issue.Message)
				}
			}

			if report.DaemonStatus.Running {
				logger.OK("Daemon: running (PID %d)", report.DaemonStatus.PID)
			} else {
				logger.OK("Daemon: not running")
			}

			if !report.Healthy {
				return fmt.Errorf("ecosystem has issues")
			}
			logger.OK("Ecosystem healthy")
			return nil
		},
	}

	return cmd
}
