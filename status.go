package phonewave

import (
	"os"
	"path/filepath"
)

// StatusReport holds daemon and ecosystem status information.
type StatusReport struct {
	DaemonRunning bool
	DaemonPID     int
	OutboxCount   int
	RouteCount    int
	RepoCount     int
	PendingErrors int
}

// Status collects current daemon and ecosystem status.
func Status(cfg *Config, stateDir string) StatusReport {
	report := StatusReport{
		RouteCount: len(cfg.Routes),
		RepoCount:  len(cfg.Repositories),
	}

	// Count outbox directories (endpoints with produces or consumes)
	for _, repo := range cfg.Repositories {
		for _, ep := range repo.Endpoints {
			if len(ep.Produces) > 0 || len(ep.Consumes) > 0 {
				report.OutboxCount++
			}
		}
	}

	// Check daemon status
	daemonStatus := checkDaemonStatus(stateDir)
	report.DaemonRunning = daemonStatus.Running
	report.DaemonPID = daemonStatus.PID

	// Count pending error files
	errorsDir := filepath.Join(stateDir, "errors")
	entries, err := os.ReadDir(errorsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				report.PendingErrors++
			}
		}
	}

	return report
}
