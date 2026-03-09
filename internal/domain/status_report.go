package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StatusReport holds daemon and ecosystem status information.
type StatusReport struct {
	DaemonRunning     bool          `json:"daemon_running"`
	DaemonPID         int           `json:"daemon_pid,omitempty"`
	OutboxCount       int           `json:"outbox_count"`
	RouteCount        int           `json:"route_count"`
	RepoCount         int           `json:"repo_count"`
	PendingErrors     int           `json:"pending_errors"`
	Uptime            time.Duration `json:"uptime"`
	DeliveredCount24h int           `json:"delivered_24h"`
	FailedCount24h    int           `json:"failed_24h"`
	RetriedCount24h   int           `json:"retried_24h"`
	SuccessRate24h    float64       `json:"success_rate_24h"`
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
	fmt.Fprintf(&b, "  %-16s %d delivered, %d failed, %d retried\n",
		"Last 24h:", r.DeliveredCount24h, r.FailedCount24h, r.RetriedCount24h)
	fmt.Fprintf(&b, "  %-16s %s\n", "Success:",
		FormatSuccessRate(r.SuccessRate24h, r.DeliveredCount24h,
			r.DeliveredCount24h+r.FailedCount24h))

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
