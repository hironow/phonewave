package phonewave

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DoctorIssue represents a single health check finding.
type DoctorIssue struct {
	Endpoint string // e.g. "repo-a/.siren" or "" for global issues
	Message  string
	Severity string // "error", "warn", "fixed", "ok"
}

// DaemonHealthStatus holds daemon-related health info.
type DaemonHealthStatus struct {
	Checked bool
	Running bool
	PID     int
}

// EndpointHealth holds health info for a single endpoint.
type EndpointHealth struct {
	Repo     string
	Dir      string
	Produces []string
	Consumes []string
	OK       bool
}

// DoctorReport holds the complete health check result.
type DoctorReport struct {
	Healthy      bool
	Issues       []DoctorIssue
	Endpoints    []EndpointHealth
	DaemonStatus DaemonHealthStatus
}

// Doctor verifies ecosystem health and returns a report.
func Doctor(cfg *Config, stateDir string) DoctorReport {
	report := DoctorReport{
		Healthy: true,
		DaemonStatus: DaemonHealthStatus{
			Checked: true,
		},
	}

	// Check each repository
	for _, repo := range cfg.Repositories {
		// 1. Verify repository path exists
		if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
			report.addError("", fmt.Sprintf("Repository path does not exist: %s", repo.Path))
			continue
		}

		// 2. Check each endpoint
		for _, ep := range repo.Endpoints {
			epLabel := fmt.Sprintf("%s/%s", filepath.Base(repo.Path), ep.Dir)
			epHealth := EndpointHealth{
				Repo:     repo.Path,
				Dir:      ep.Dir,
				Produces: ep.Produces,
				Consumes: ep.Consumes,
				OK:       true,
			}

			// Check endpoint directory exists
			epDir := filepath.Join(repo.Path, ep.Dir)
			if _, err := os.Stat(epDir); os.IsNotExist(err) {
				report.addError(epLabel, fmt.Sprintf("Endpoint directory missing: %s", epDir))
				epHealth.OK = false
				report.Endpoints = append(report.Endpoints, epHealth)
				continue
			}

			// Check and auto-create outbox/inbox
			for _, sub := range []string{"outbox", "inbox"} {
				subDir := filepath.Join(epDir, sub)
				if _, err := os.Stat(subDir); os.IsNotExist(err) {
					if err := os.MkdirAll(subDir, 0755); err != nil {
						report.addError(epLabel, fmt.Sprintf("Failed to create %s/: %v", sub, err))
						epHealth.OK = false
					} else {
						report.addFixed(epLabel, fmt.Sprintf("Created missing %s/ directory", sub))
					}
				}
			}

			// Verify SKILL.md files are parseable
			if len(ep.Produces) > 0 {
				skillPath := filepath.Join(epDir, "skills", "dmail-sendable", "SKILL.md")
				if data, err := os.ReadFile(skillPath); err == nil {
					if _, err := ParseSkillFrontmatter(data); err != nil {
						report.addWarn(epLabel, fmt.Sprintf("dmail-sendable SKILL.md parse error: %v", err))
					}
				}
			}
			if len(ep.Consumes) > 0 {
				skillPath := filepath.Join(epDir, "skills", "dmail-readable", "SKILL.md")
				if data, err := os.ReadFile(skillPath); err == nil {
					if _, err := ParseSkillFrontmatter(data); err != nil {
						report.addWarn(epLabel, fmt.Sprintf("dmail-readable SKILL.md parse error: %v", err))
					}
				}
			}

			report.addOK(epLabel, fmt.Sprintf("OK (produces: %v, consumes: %v)", ep.Produces, ep.Consumes))
			report.Endpoints = append(report.Endpoints, epHealth)
		}
	}

	// Check orphaned routes (per-repo to match routing scope)
	orphans := DetectOrphansPerRepo(cfg)
	for _, kind := range orphans.UnconsumedKinds {
		report.addWarn("", fmt.Sprintf("Orphaned: kind=%q is produced but not consumed", kind))
	}
	for _, kind := range orphans.UnproducedKinds {
		report.addWarn("", fmt.Sprintf("Orphaned: kind=%q is consumed but not produced", kind))
	}

	// Check daemon status
	report.DaemonStatus = checkDaemonStatus(stateDir)

	return report
}

func checkDaemonStatus(stateDir string) DaemonHealthStatus {
	status := DaemonHealthStatus{Checked: true}

	pidPath := filepath.Join(stateDir, "watch.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return status // Not running (no PID file)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return status
	}

	// Check if process is actually running
	process, err := os.FindProcess(pid)
	if err != nil {
		return status
	}

	// On Unix, FindProcess always succeeds. Send signal 0 to check.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return status // Process not running (stale PID file)
	}

	status.Running = true
	status.PID = pid
	return status
}

func (r *DoctorReport) addError(endpoint, msg string) {
	r.Healthy = false
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "error"})
}

func (r *DoctorReport) addWarn(endpoint, msg string) {
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "warn"})
}

func (r *DoctorReport) addFixed(endpoint, msg string) {
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "fixed"})
}

func (r *DoctorReport) addOK(endpoint, msg string) {
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "ok"})
}
