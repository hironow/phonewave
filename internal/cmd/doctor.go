package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify ecosystem health",
		Long:  "Check ecosystem health: verify paths, endpoints, PID conflicts, and daemon status.",
		Args:  cobra.NoArgs,
		Example: `  phonewave doctor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := configPath(cmd)
			cfg, err := phonewave.LoadConfig(cfgPath)
			if err != nil {
				phonewave.LogInfo("Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			stateDir := filepath.Join(configBase(cmd), phonewave.StateDir)
			report := phonewave.Doctor(cfg, stateDir)

			for _, issue := range report.Issues {
				switch issue.Severity {
				case "ok":
					phonewave.LogOK("%s  %s", issue.Endpoint, issue.Message)
				case "fixed":
					phonewave.LogWarn("%s  %s", issue.Endpoint, issue.Message)
				case "warn":
					phonewave.LogWarn("%s  %s", issue.Endpoint, issue.Message)
				case "error":
					phonewave.LogError("%s  %s", issue.Endpoint, issue.Message)
				}
			}

			if report.DaemonStatus.Running {
				phonewave.LogOK("Daemon: running (PID %d)", report.DaemonStatus.PID)
			} else {
				phonewave.LogOK("Daemon: not running")
			}

			if !report.Healthy {
				return fmt.Errorf("ecosystem has issues")
			}
			phonewave.LogOK("Ecosystem healthy")
			return nil
		},
	}
}
