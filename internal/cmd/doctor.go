package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/session"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [repo-path]",
		Short: "Verify ecosystem health",
		Long: `Check ecosystem health: verify paths, endpoints, SKILL.md spec compliance, PID conflicts, and daemon status.

With --all, runs all 4 tool doctors (sightjack, paintress, amadeus) against
the specified repo path and presents a unified report with cross-tool checks.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  # Run phonewave-only health check
  phonewave doctor

  # Run unified health check across all 4 tools
  phonewave doctor --all /path/to/repo

  # JSON output for scripting
  phonewave doctor -o json

  # Auto-fix repairable issues
  phonewave doctor --repair`,
		RunE: func(cmd *cobra.Command, args []string) error {
			allMode, _ := cmd.Flags().GetBool("all")
			outputFmt, _ := cmd.Flags().GetString("output")
			jsonOut := outputFmt == "json"

			if allMode {
				repoPath := ""
				if len(args) > 0 {
					repoPath = args[0]
				}
				return runUnifiedDoctor(cmd, repoPath, jsonOut)
			}

			repair, _ := cmd.Flags().GetBool("repair")

			cfgPath := configPath(cmd)
			stateDir := configBase(cmd)

			cfg, err := session.LoadConfig(cfgPath)
			if err != nil {
				w := cmd.ErrOrStderr()
				earlyLogger := platform.NewLogger(w, false)
				failLabel := earlyLogger.Colorize(fmt.Sprintf("%-4s", "FAIL"), platform.SeverityColor("error"))
				fmt.Fprintf(w, "  [%s] %-16s %s\n", failLabel, "config", "Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			report := session.Doctor(cfg, stateDir, repair, cfgPath)

			if jsonOut {
				data, err := session.FormatDoctorJSON(report)
				if err != nil {
					return fmt.Errorf("format JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				if !report.Healthy {
					return &domain.SilentError{Err: fmt.Errorf("ecosystem has issues")}
				}
				return nil
			}

			// text output — aligned with amadeus/sightjack/paintress format
			w := cmd.ErrOrStderr()
			logger := platform.NewLogger(w, false)
			fmt.Fprintln(w, "phonewave doctor — ecosystem health check")
			fmt.Fprintln(w)

			var fails, warns int
			for _, issue := range report.Issues {
				status := severityToStatus(issue.Severity)
				name := issue.Endpoint
				if name == "" {
					name = "-"
				}

				label := logger.Colorize(fmt.Sprintf("%-4s", status), platform.SeverityColor(issue.Severity))
				fmt.Fprintf(w, "  [%s] %-16s %s\n", label, name, issue.Message)
				if issue.Hint != "" {
					fmt.Fprintf(w, "         %-16s hint: %s\n", "", issue.Hint)
				}

				switch issue.Severity {
				case "error":
					fails++
				case "warn":
					warns++
				}
			}

			// Daemon status
			daemonLabel := logger.Colorize(fmt.Sprintf("%-4s", "OK"), platform.SeverityColor("ok"))
			if report.DaemonStatus.Running {
				fmt.Fprintf(w, "  [%s] %-16s running (PID %d)\n", daemonLabel, "daemon", report.DaemonStatus.PID)
			} else {
				fmt.Fprintf(w, "  [%s] %-16s not running\n", daemonLabel, "daemon")
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
				return &domain.SilentError{Err: fmt.Errorf("ecosystem has issues")}
			}
			fmt.Fprintln(w, "All checks passed.")
			return nil
		},
	}

	cmd.Flags().Bool("repair", false, "Auto-fix repairable issues")
	cmd.Flags().Bool("all", false, "Run unified doctor across all 4 TAP tools")

	return cmd
}

// runUnifiedDoctor orchestrates all 4 tool doctors and presents a unified report.
func runUnifiedDoctor(cmd *cobra.Command, repoPath string, jsonOut bool) error {
	ctx := cmd.Context()
	cfgPath := configPath(cmd)
	cfg, _ := session.LoadConfig(cfgPath) // best-effort: crosscheck needs config

	// Run tool doctors in parallel
	tools := []string{"phonewave", "sightjack", "paintress", "amadeus"}
	sections := make([]domain.ToolSection, len(tools))
	var wg sync.WaitGroup
	for i, tool := range tools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sections[i] = session.RunToolDoctor(ctx, tool, repoPath)
		}()
	}
	wg.Wait()

	// Cross-tool checks
	crossChecks := session.CheckRoutingConsistency(cfg)

	report := domain.UnifiedDoctorReport{
		Sections:  sections,
		CrossTool: crossChecks,
	}
	report.Healthy = report.IsHealthy()

	if jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("format JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		if !report.Healthy {
			return &domain.SilentError{Err: fmt.Errorf("unified doctor: issues found")}
		}
		return nil
	}

	// Text output
	w := cmd.ErrOrStderr()
	logger := platform.NewLogger(w, false)
	fmt.Fprintln(w, "phonewave doctor --all — TAP ecosystem health check")
	fmt.Fprintln(w)

	var totalFails, totalWarns int
	for _, sec := range report.Sections {
		header := fmt.Sprintf("[%s", sec.Tool)
		if sec.Path != "" {
			header += " " + sec.Path
		}
		header += "]"
		fmt.Fprintf(w, "  %s\n", header)

		if sec.Error != "" {
			label := logger.Colorize(fmt.Sprintf("%-4s", "FAIL"), platform.SeverityColor("error"))
			fmt.Fprintf(w, "  [%s] %-16s %s\n", label, "exec", sec.Error)
			totalFails++
			continue
		}

		for _, c := range sec.Checks {
			sev := statusToSeverity(c.Status)
			label := logger.Colorize(fmt.Sprintf("%-4s", c.Status), platform.SeverityColor(sev))
			fmt.Fprintf(w, "  [%s] %-16s %s\n", label, c.Name, c.Message)
			if c.Hint != "" {
				fmt.Fprintf(w, "         %-16s hint: %s\n", "", c.Hint)
			}
			switch c.Status {
			case "FAIL":
				totalFails++
			case "WARN":
				totalWarns++
			}
		}
		fmt.Fprintln(w)
	}

	if len(report.CrossTool) > 0 {
		fmt.Fprintln(w, "  [cross-tool]")
		for _, c := range report.CrossTool {
			sev := statusToSeverity(c.Status)
			label := logger.Colorize(fmt.Sprintf("%-4s", c.Status), platform.SeverityColor(sev))
			fmt.Fprintf(w, "  [%s] %-16s %s\n", label, c.Name, c.Message)
			if c.Hint != "" {
				fmt.Fprintf(w, "         %-16s hint: %s\n", "", c.Hint)
			}
			switch c.Status {
			case "FAIL":
				totalFails++
			case "WARN":
				totalWarns++
			}
		}
		fmt.Fprintln(w)
	}

	if !report.Healthy {
		var parts []string
		if totalFails > 0 {
			parts = append(parts, fmt.Sprintf("%d error(s)", totalFails))
		}
		if totalWarns > 0 {
			parts = append(parts, fmt.Sprintf("%d warning(s)", totalWarns))
		}
		fmt.Fprintln(w, strings.Join(parts, ", ")+".")
		return &domain.SilentError{Err: fmt.Errorf("unified doctor: issues found")}
	}
	fmt.Fprintln(w, "All checks passed.")
	return nil
}

// statusToSeverity converts UnifiedCheck status to phonewave severity for coloring.
func statusToSeverity(status string) string {
	switch status {
	case "FAIL":
		return "error"
	case "WARN":
		return "warn"
	case "FIX":
		return "fixed"
	default:
		return "ok"
	}
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
