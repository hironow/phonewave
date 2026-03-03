package session

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

// DeliveryStats24h holds delivery statistics from the last 24 hours.
type DeliveryStats24h struct {
	Delivered int
	Failed    int
	Retried   int
}

// ParseDeliveryStats reads delivery.log and counts actions from the last 24 hours.
func ParseDeliveryStats(stateDir string) DeliveryStats24h {
	logPath := filepath.Join(stateDir, "delivery.log")
	f, err := os.Open(logPath)
	if err != nil {
		return DeliveryStats24h{}
	}
	defer f.Close()

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	var stats DeliveryStats24h

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 20 {
			continue
		}

		// Parse timestamp (RFC3339 at start of line)
		spaceIdx := strings.IndexByte(line, ' ')
		if spaceIdx < 0 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, line[:spaceIdx])
		if err != nil {
			continue
		}
		if ts.Before(cutoff) {
			continue
		}

		// Parse action (after timestamp + space)
		rest := strings.TrimSpace(line[spaceIdx+1:])
		actionEnd := strings.IndexByte(rest, ' ')
		var action string
		if actionEnd < 0 {
			action = rest
		} else {
			action = rest[:actionEnd]
		}

		switch action {
		case "DELIVERED":
			stats.Delivered++
		case "FAILED":
			stats.Failed++
		case "RETRIED":
			stats.Retried++
		}
	}

	return stats
}

// Status collects current daemon and ecosystem status.
func Status(cfg *domain.Config, stateDir string) domain.StatusReport {
	report := domain.StatusReport{
		RouteCount: len(cfg.Routes),
		RepoCount:  len(cfg.Repositories),
	}

	// Count outbox directories (only endpoints that produce, matching CollectOutboxDirs)
	for _, repo := range cfg.Repositories {
		for _, ep := range repo.Endpoints {
			if len(ep.Produces) > 0 {
				report.OutboxCount++
			}
		}
	}

	// Check daemon status
	daemonStatus := checkDaemonStatus(stateDir)
	report.DaemonRunning = daemonStatus.Running
	report.DaemonPID = daemonStatus.PID

	// Read uptime from watch.started
	startedPath := filepath.Join(stateDir, "watch.started")
	if data, err := os.ReadFile(startedPath); err == nil {
		if startTime, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			report.Uptime = time.Since(startTime)
		}
	}

	// Delivery stats from last 24h
	stats := ParseDeliveryStats(stateDir)
	report.DeliveredCount24h = stats.Delivered
	report.FailedCount24h = stats.Failed
	report.RetriedCount24h = stats.Retried
	report.SuccessRate24h = domain.DeliveryMetrics{
		Delivered: stats.Delivered,
		Failed:    stats.Failed,
	}.SuccessRate()

	// Count pending error files (exclude .err sidecars to avoid 2x count)
	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".err") {
				report.PendingErrors++
			}
		}
	}

	return report
}
