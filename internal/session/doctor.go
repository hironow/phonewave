package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hironow/phonewave/internal/domain"
)

// DoctorIssue represents a single health check finding.
type DoctorIssue struct {
	Endpoint string `json:"endpoint"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warn", "fixed", "ok"
}

// DaemonHealthStatus holds daemon-related health info.
type DaemonHealthStatus struct {
	Checked bool `json:"checked"`
	Running bool `json:"running"`
	PID     int  `json:"pid,omitempty"`
}

// EndpointHealth holds health info for a single endpoint.
type EndpointHealth struct {
	Repo     string   `json:"repo"`
	Dir      string   `json:"dir"`
	Produces []string `json:"produces,omitempty"`
	Consumes []string `json:"consumes,omitempty"`
	OK       bool     `json:"ok"`
}

// DoctorReport holds the complete health check result.
type DoctorReport struct {
	Healthy      bool               `json:"healthy"`
	Issues       []DoctorIssue      `json:"issues"`
	Endpoints    []EndpointHealth   `json:"endpoints,omitempty"`
	DaemonStatus DaemonHealthStatus `json:"daemon_status"`
}

// FormatDoctorJSON marshals a DoctorReport to indented JSON.
func FormatDoctorJSON(report DoctorReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// Doctor verifies ecosystem health and returns a report.
func Doctor(cfg *domain.Config, stateDir string) DoctorReport {
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

			// Verify SKILL.md files are parseable and spec-compliant
			for _, skillName := range []string{SkillSendable, SkillReadable} {
				skillDir := filepath.Join(epDir, "skills", skillName)
				skillPath := filepath.Join(skillDir, "SKILL.md")
				data, err := os.ReadFile(skillPath)
				if err != nil {
					if os.IsNotExist(err) {
						continue // SKILL.md does not exist; skip
					}
					report.addWarn(epLabel, fmt.Sprintf("Failed to read %s SKILL.md: %v", skillName, err))
					continue
				}
				if _, err := ParseSkillFrontmatter(data); err != nil {
					report.addWarn(epLabel, fmt.Sprintf("%s SKILL.md parse error: %v", skillName, err))
				}
				// Run skills-ref spec compliance check (best-effort)
				if problems, err := ValidateSkillDir(skillDir); err != nil {
					report.addWarn(epLabel, fmt.Sprintf("skills-ref validate %s: %v", skillName, err))
				} else if len(problems) > 0 {
					for _, p := range problems {
						report.addWarn(epLabel, fmt.Sprintf("skills-ref: %s", p))
					}
				}
			}

			report.addOK(epLabel, fmt.Sprintf("OK (produces: %v, consumes: %v)", ep.Produces, ep.Consumes))
			report.Endpoints = append(report.Endpoints, epHealth)
		}
	}

	// Check orphaned routes (per-repo to match routing scope)
	orphans := domain.DetectOrphansPerRepo(cfg)
	for _, kind := range orphans.UnconsumedKinds {
		report.addWarn("", fmt.Sprintf("Orphaned: kind=%q is produced but not consumed", kind))
	}
	for _, kind := range orphans.UnproducedKinds {
		report.addWarn("", fmt.Sprintf("Orphaned: kind=%q is consumed but not produced", kind))
	}

	// Success rate (informational)
	stats := ParseDeliveryStats(stateDir)
	m := domain.DeliveryMetrics{Delivered: stats.Delivered, Failed: stats.Failed}
	report.addOK("success-rate", domain.FormatSuccessRate(m.SuccessRate(), stats.Delivered, stats.Delivered+stats.Failed))

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
