package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
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
			allMode := mustBool(cmd, "all")
			outputFmt := mustString(cmd, "output")
			jsonOut := outputFmt == "json"

			if allMode {
				repoPath := ""
				if len(args) > 0 {
					repoPath = args[0]
				}
				return runUnifiedDoctor(cmd, repoPath, jsonOut)
			}

			// Reject positional repo-path without --all to avoid silent misreport
			if len(args) > 0 {
				return fmt.Errorf("repo-path argument requires --all flag")
			}

			repair := mustBool(cmd, "repair")

			cfgPath := configPath(cmd)
			stateDir := configBase(cmd)

			cfg, err := session.LoadConfig(cfgPath)
			if err != nil {
				w := cmd.ErrOrStderr()
				earlyLogger := platform.NewLogger(w, false)
				failLabel := earlyLogger.Colorize(fmt.Sprintf("%-4s", "FAIL"), platform.StatusColor(domain.CheckFail))
				fmt.Fprintf(w, "  [%s] %-16s %s\n", failLabel, "config", "Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			report := session.Doctor(cmd.Context(), cfg, stateDir, repair, cfgPath)

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
			for _, check := range report.Checks {
				name := check.Name
				if name == "" {
					name = "-"
				}
				label := logger.Colorize(fmt.Sprintf("%-4s", check.Status.StatusLabel()), platform.StatusColor(check.Status))
				fmt.Fprintf(w, "  [%s] %-16s %s\n", label, name, check.Message)
				if check.Hint != "" {
					fmt.Fprintf(w, "         %-16s hint: %s\n", "", check.Hint)
				}
				switch check.Status {
				case domain.CheckFail:
					fails++
				case domain.CheckWarn:
					warns++
				}
			}

			// Daemon status
			daemonLabel := logger.Colorize(fmt.Sprintf("%-4s", "OK"), platform.StatusColor(domain.CheckOK))
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

	// Load config strictly from the target repo path (not cwd fallback).
	// If repoPath has no phonewave config, report WARN — don't mix repos.
	repair := mustBool(cmd, "repair")
	var cfg *domain.Config
	var cfgPath string
	var stateDir string
	if repoPath != "" {
		cfgPath = filepath.Join(repoPath, domain.StateDir, "config.yaml")
	} else {
		cfgPath = configPath(cmd)
	}
	stateDir = filepath.Dir(cfgPath)
	loadedCfg, cfgErr := session.LoadConfig(cfgPath)
	if cfgErr == nil {
		cfg = loadedCfg
	}

	// Run phonewave doctor locally (not via subprocess) to use the correct config.
	// If config is missing, report as a WARN check instead of crashing.
	var pwReport domain.DoctorReport
	if cfg == nil {
		pwReport = domain.DoctorReport{Healthy: true}
		pwReport.AddWarn("config", fmt.Sprintf("phonewave not initialized (%s)", cfgPath))
	} else {
		pwReport = session.Doctor(cmd.Context(), cfg, stateDir, repair, cfgPath)
	}
	pwSection := domain.ToolSection{Tool: "phonewave"}
	for _, check := range pwReport.Checks {
		pwSection.Checks = append(pwSection.Checks, domain.UnifiedCheck{
			Name:    check.Name,
			Status:  check.Status.StatusLabel(),
			Message: check.Message,
			Hint:    check.Hint,
		})
	}
	daemonMsg := "not running"
	if pwReport.DaemonStatus.Running {
		daemonMsg = fmt.Sprintf("running (PID %d)", pwReport.DaemonStatus.PID)
	}
	pwSection.Checks = append(pwSection.Checks, domain.UnifiedCheck{
		Name: "daemon", Status: "OK", Message: daemonMsg,
	})

	// Resolve repo paths per tool from phonewave config (multi-repo support).
	// Maps endpoint state dir to tool name for lookup.
	toolForDir := map[string]string{
		".siren": "sightjack", ".expedition": "paintress", ".gate": "amadeus",
	}
	toolPaths := make(map[string]string) // tool -> repo path
	if cfg != nil {
		for _, repo := range cfg.Repositories {
			for _, ep := range repo.Endpoints {
				if tool, ok := toolForDir[ep.Dir]; ok {
					toolPaths[tool] = repo.Path
				}
			}
		}
	}
	// Fallback: if config doesn't have a path for a tool, use repoPath
	otherTools := []string{"sightjack", "paintress", "amadeus"}
	for _, tool := range otherTools {
		if _, ok := toolPaths[tool]; !ok {
			toolPaths[tool] = repoPath
		}
	}

	// Run other 3 tool doctors in parallel via subprocess
	otherSections := make([]domain.ToolSection, len(otherTools))
	var wg sync.WaitGroup
	for i, tool := range otherTools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			otherSections[i] = session.RunToolDoctor(ctx, tool, toolPaths[tool], repair)
		}()
	}
	wg.Wait()

	sections := append([]domain.ToolSection{pwSection}, otherSections...)

	// Cross-tool checks (uses config loaded from target repo)
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
			label := logger.Colorize(fmt.Sprintf("%-4s", "FAIL"), platform.StatusColorFromLabel("FAIL"))
			fmt.Fprintf(w, "  [%s] %-16s %s\n", label, "exec", sec.Error)
			totalFails++
			continue
		}

		for _, c := range sec.Checks {
			label := logger.Colorize(fmt.Sprintf("%-4s", c.Status), platform.StatusColorFromLabel(c.Status))
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
			label := logger.Colorize(fmt.Sprintf("%-4s", c.Status), platform.StatusColorFromLabel(c.Status))
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
