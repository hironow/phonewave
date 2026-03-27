package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

// toolDoctorTimeout is the maximum time to wait for a tool's doctor command.
const toolDoctorTimeout = 30 * time.Second

// runStaticToolDoctor runs a specific tool's doctor command with static binary name.
// This avoids dynamic exec.Command which semgrep flags as code injection risk.
func runStaticToolDoctor(ctx context.Context, tool string, repoPath string) ([]byte, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, toolDoctorTimeout)
	defer cancel()

	switch tool {
	case "phonewave":
		cmd := exec.CommandContext(cmdCtx, "phonewave", "doctor", "-o", "json")
		return cmd.Output()
	case "sightjack":
		cmd := exec.CommandContext(cmdCtx, "sightjack", "doctor", "-j", repoPath)
		return cmd.Output()
	case "paintress":
		cmd := exec.CommandContext(cmdCtx, "paintress", "doctor", "-o", "json", repoPath)
		return cmd.Output()
	case "amadeus":
		cmd := exec.CommandContext(cmdCtx, "amadeus", "doctor", "-j", repoPath)
		return cmd.Output()
	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
}

// RunToolDoctor executes a tool's doctor command and parses the JSON output
// into a ToolSection. Handles both JSON formats:
//   - phonewave: {"healthy":bool,"issues":[{endpoint,message,severity}],...}
//   - sightjack/paintress/amadeus: [{"name":...,"status":...,"message":...,"hint":...}]
//
// Tools exit non-zero when checks fail but still produce valid JSON.
// Only treat as error when no JSON output is available.
func RunToolDoctor(ctx context.Context, tool string, repoPath string) domain.ToolSection {
	section := domain.ToolSection{Tool: tool, Path: repoPath}

	out, err := runStaticToolDoctor(ctx, tool, repoPath)

	// Tools exit non-zero on FAIL checks but still produce JSON.
	// Only error when no output at all.
	if err != nil && len(out) == 0 {
		section.Error = fmt.Sprintf("exec %s: %v", tool, err)
		return section
	}

	// Try format 1: array of checks (sightjack/paintress/amadeus)
	var checks []domain.UnifiedCheck
	if jsonErr := json.Unmarshal(out, &checks); jsonErr == nil && len(checks) > 0 {
		section.Checks = checks
		return section
	}

	// Try format 2: phonewave DoctorReport
	var pwReport struct {
		Healthy bool `json:"healthy"`
		Issues  []struct {
			Endpoint string `json:"endpoint"`
			Message  string `json:"message"`
			Severity string `json:"severity"`
			Hint     string `json:"hint,omitempty"`
		} `json:"issues"`
		DaemonStatus struct {
			Running bool `json:"running"`
			PID     int  `json:"pid"`
		} `json:"daemon_status"`
	}
	if jsonErr := json.Unmarshal(out, &pwReport); jsonErr == nil {
		for _, issue := range pwReport.Issues {
			status := "OK"
			switch issue.Severity {
			case "error":
				status = "FAIL"
			case "warn":
				status = "WARN"
			case "fixed":
				status = "FIX"
			}
			name := issue.Endpoint
			if name == "" {
				name = "-"
			}
			section.Checks = append(section.Checks, domain.UnifiedCheck{
				Name:    name,
				Status:  status,
				Message: issue.Message,
				Hint:    issue.Hint,
			})
		}
		// Add daemon status as a check
		daemonMsg := "not running"
		if pwReport.DaemonStatus.Running {
			daemonMsg = fmt.Sprintf("running (PID %d)", pwReport.DaemonStatus.PID)
		}
		section.Checks = append(section.Checks, domain.UnifiedCheck{
			Name:    "daemon",
			Status:  "OK",
			Message: daemonMsg,
		})
		return section
	}

	// Neither format matched
	section.Error = "failed to parse doctor JSON output"
	return section
}
