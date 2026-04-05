package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StatusReport holds daemon and ecosystem status information.
type StatusReport struct {
	DaemonRunning       bool          `json:"daemon_running"`
	DaemonPID           int           `json:"daemon_pid,omitempty"`
	OutboxCount         int           `json:"outbox_count"`
	RouteCount          int           `json:"route_count"`
	RepoCount           int           `json:"repo_count"`
	PendingErrors       int           `json:"pending_errors"`
	ProviderState       string        `json:"provider_state,omitempty"`
	ProviderReason      string        `json:"provider_reason,omitempty"`
	ProviderRetryBudget int           `json:"provider_retry_budget,omitempty"`
	ProviderResumeAt    time.Time     `json:"provider_resume_at,omitempty"`
	ProviderResumeWhen  string        `json:"provider_resume_when,omitempty"`
	Uptime              time.Duration `json:"uptime"`
	DeliveredCount24h   int           `json:"delivered_24h"`
	FailedCount24h      int           `json:"failed_24h"`
	RetriedCount24h     int           `json:"retried_24h"`
	SuccessRate24h      float64       `json:"success_rate_24h"`
	SkillsRefVenv       string        `json:"skills_ref_venv,omitempty"`
	SkillsRefReady      bool          `json:"skills_ref_ready"`
}

// FormatText returns a human-readable status report string suitable for stdout.
func (r StatusReport) FormatText() string {
	var b strings.Builder
	b.WriteString("phonewave status\n\n")

	// Daemon
	if r.DaemonRunning {
		fmt.Fprintf(&b, "  %-16s running (PID %d)\n", "Daemon:", r.DaemonPID)
	} else {
		fmt.Fprintf(&b, "  %-16s stopped\n", "Daemon:")
	}
	if r.Uptime > 0 {
		fmt.Fprintf(&b, "  %-16s %s\n", "Uptime:", r.Uptime.Truncate(time.Second))
	}

	fmt.Fprintf(&b, "  %-16s %d outbox dirs across %d repos\n", "Watching:", r.OutboxCount, r.RepoCount)
	fmt.Fprintf(&b, "  %-16s %d\n", "Routes:", r.RouteCount)
	fmt.Fprintf(&b, "  %-16s %d items in error queue\n", "Pending:", r.PendingErrors)
	if r.ProviderState != "" {
		fmt.Fprintf(&b, "  %-16s %s", "Provider:", r.ProviderState)
		if r.ProviderReason != "" {
			fmt.Fprintf(&b, " (%s)", r.ProviderReason)
		}
		b.WriteByte('\n')
		if r.ProviderRetryBudget > 0 {
			fmt.Fprintf(&b, "  %-16s %d\n", "Retry budget:", r.ProviderRetryBudget)
		}
		if r.ProviderResumeWhen != "" {
			fmt.Fprintf(&b, "  %-16s %s\n", "Resume when:", r.ProviderResumeWhen)
		}
		if !r.ProviderResumeAt.IsZero() {
			fmt.Fprintf(&b, "  %-16s %s\n", "Resume at:", r.ProviderResumeAt.UTC().Format(time.RFC3339))
		}
	}
	fmt.Fprintf(&b, "  %-16s %d delivered, %d failed, %d retried\n",
		"Last 24h:", r.DeliveredCount24h, r.FailedCount24h, r.RetriedCount24h)
	fmt.Fprintf(&b, "  %-16s %s\n", "Success:",
		FormatSuccessRate(r.SuccessRate24h, r.DeliveredCount24h,
			r.DeliveredCount24h+r.FailedCount24h))

	// Skills-ref toolchain
	if r.SkillsRefReady {
		fmt.Fprintf(&b, "  %-16s ready (venv: %s)\n", "Skills-ref:", r.SkillsRefVenv)
	} else {
		fmt.Fprintf(&b, "  %-16s not available\n", "Skills-ref:")
	}

	return b.String()
}

// FormatJSON returns the status report as a compact JSON string.
func (r StatusReport) FormatJSON() string {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}
