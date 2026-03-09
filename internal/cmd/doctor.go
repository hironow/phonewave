package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
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
			outputFmt, _ := cmd.Flags().GetString("output")
			jsonOut := outputFmt == "json"

			cfgPath := configPath(cmd)
			stateDir := filepath.Join(configBase(cmd), domain.StateDir)

			cfg, err := session.LoadConfig(cfgPath)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "  [FAIL] config           Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			report := session.Doctor(cfg, stateDir)

			if jsonOut {
				data, err := session.FormatDoctorJSON(report)
				if err != nil {
					return fmt.Errorf("format JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				if !report.Healthy {
					return fmt.Errorf("ecosystem has issues")
				}
				return nil
			}

			// text output — aligned with amadeus/sightjack/paintress format
			w := cmd.ErrOrStderr()
			fmt.Fprintln(w, "phonewave doctor — ecosystem health check")
			fmt.Fprintln(w)

			var fails, warns int
			for _, issue := range report.Issues {
				status := severityToStatus(issue.Severity)
				label := issue.Message
				if issue.Endpoint != "" {
					label = issue.Endpoint + "  " + label
				}

				fmt.Fprintf(w, "  [%-4s] %s\n", status, label)
				if issue.Hint != "" {
					fmt.Fprintf(w, "         hint: %s\n", issue.Hint)
				}

				switch issue.Severity {
				case "error":
					fails++
				case "warn":
					warns++
				}
			}

			// Daemon status
			if report.DaemonStatus.Running {
				fmt.Fprintf(w, "  [%-4s] daemon: running (PID %d)\n", "OK", report.DaemonStatus.PID)
			} else {
				fmt.Fprintf(w, "  [%-4s] daemon: not running\n", "OK")
			}

			fmt.Fprintln(w)
			if !report.Healthy {
				var parts []string
				if fails > 0 {
					parts = append(parts, fmt.Sprintf("%d error(s)", fails))
				}
				if warns > 0 {
					parts = append(parts, fmt.Sprintf("%d warning(s)", warns))
				}
				fmt.Fprintln(w, strings.Join(parts, ", ")+".")
				return fmt.Errorf("ecosystem has issues")
			}
			fmt.Fprintln(w, "All checks passed.")
			return nil
		},
	}

	return cmd
}

// severityToStatus maps phonewave DoctorIssue severity to [FAIL]/[OK]/[WARN]/[FIX] labels.
func severityToStatus(severity string) string {
	switch severity {
	case "error":
		return "FAIL"
	case "warn":
		return "WARN"
	case "fixed":
		return "FIX"
	default:
		return "OK"
	}
}
