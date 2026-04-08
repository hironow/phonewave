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
func runStaticToolDoctor(ctx context.Context, tool string, repoPath string, repair bool) ([]byte, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, toolDoctorTimeout)
	defer cancel()

	switch tool {
	case "sightjack":
		args := []string{"doctor", "-j"}
		if repair {
			args = append(args, "--repair")
		}
		args = append(args, repoPath)
		cmd := exec.CommandContext(cmdCtx, "sightjack", args...)
		return cmd.Output()
	case "paintress":
		args := []string{"doctor", "-o", "json"}
		if repair {
			args = append(args, "--repair")
		}
		args = append(args, repoPath)
		cmd := exec.CommandContext(cmdCtx, "paintress", args...)
		return cmd.Output()
	case "amadeus":
		args := []string{"doctor", "-j"}
		if repair {
			args = append(args, "--repair")
		}
		args = append(args, repoPath)
		cmd := exec.CommandContext(cmdCtx, "amadeus", args...)
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
func RunToolDoctor(ctx context.Context, tool string, repoPath string, repair bool) domain.ToolSection {
	section := domain.ToolSection{Tool: tool, Path: repoPath}

	out, err := runStaticToolDoctor(ctx, tool, repoPath, repair)

	// Tools exit non-zero on FAIL checks but still produce JSON.
	// Only error when no output at all.
	if err != nil && len(out) == 0 {
		section.Error = fmt.Sprintf("exec %s: %v", tool, err)
		return section
	}

	// Try format 1: wrapped {"checks":[...]} (sightjack/paintress/amadeus)
	var wrapped struct {
		Checks []domain.UnifiedCheck `json:"checks"`
	}
	if jsonErr := json.Unmarshal(out, &wrapped); jsonErr == nil && wrapped.Checks != nil {
		section.Checks = wrapped.Checks // may be empty (clean tool = no checks)
		return section
	}

	// Try format 2: raw array of checks (fallback)
	var checks []domain.UnifiedCheck
	if jsonErr := json.Unmarshal(out, &checks); jsonErr == nil && checks != nil {
		section.Checks = checks
		return section
	}

	// Try format 3: phonewave DoctorReport (new format: checks with status labels)
	var pwReport struct {
		Healthy bool `json:"healthy"`
		Checks  []struct {
			Name    string `json:"name"`
			Status  string `json:"status"` // "OK", "FAIL", "WARN", "SKIP", "FIX"
			Message string `json:"message"`
			Hint    string `json:"hint,omitempty"`
		} `json:"checks"`
		DaemonStatus struct {
			Running bool `json:"running"`
			PID     int  `json:"pid"`
		} `json:"daemon_status"`
	}
	if jsonErr := json.Unmarshal(out, &pwReport); jsonErr == nil && len(pwReport.Checks) > 0 {
		for _, check := range pwReport.Checks {
			name := check.Name
			if name == "" {
				name = domain.DefaultEndpointName
			}
			status := check.Status
			if status == "" {
				status = "WARN" // fail-closed: unknown status → WARN
			}
			section.Checks = append(section.Checks, domain.UnifiedCheck{
				Name:    name,
				Status:  status,
				Message: check.Message,
				Hint:    check.Hint,
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
