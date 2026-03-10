package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hironow/phonewave/internal/domain"
)

// FormatDoctorJSON marshals a DoctorReport to indented JSON.
func FormatDoctorJSON(report domain.DoctorReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// Doctor verifies ecosystem health and returns a report.
func Doctor(cfg *domain.Config, stateDir string) domain.DoctorReport {
	report := domain.DoctorReport{
		Healthy: true,
		DaemonStatus: domain.DaemonHealthStatus{
			Checked: true,
		},
	}

	// Check each repository
	for _, repo := range cfg.Repositories {
		// 1. Verify repository path exists
		if _, err := os.Stat(repo.Path); errors.Is(err, fs.ErrNotExist) {
			report.AddErrorWithHint("", fmt.Sprintf("Repository path does not exist: %s", repo.Path),
				`check config.yaml repositories.path or run "phonewave remove <path>"`)
			continue
		}

		// 2. Check each endpoint
		for _, ep := range repo.Endpoints {
			epLabel := fmt.Sprintf("%s/%s", filepath.Base(repo.Path), ep.Dir)
			epHealth := domain.EndpointHealth{
				Repo:     repo.Path,
				Dir:      ep.Dir,
				Produces: ep.Produces,
				Consumes: ep.Consumes,
				OK:       true,
			}

			// Check endpoint directory exists
			epDir := filepath.Join(repo.Path, ep.Dir)
			if _, err := os.Stat(epDir); errors.Is(err, fs.ErrNotExist) {
				report.AddErrorWithHint(epLabel, fmt.Sprintf("Endpoint directory missing: %s", epDir),
					`create the directory or run "phonewave sync" to reconcile`)
				epHealth.OK = false
				report.Endpoints = append(report.Endpoints, epHealth)
				continue
			}

			// Check and auto-create outbox/inbox
			for _, sub := range []string{"outbox", "inbox"} {
				subDir := filepath.Join(epDir, sub)
				if _, err := os.Stat(subDir); errors.Is(err, fs.ErrNotExist) {
					if err := os.MkdirAll(subDir, 0755); err != nil {
						report.AddErrorWithHint(epLabel, fmt.Sprintf("Failed to create %s/: %v", sub, err),
							"check file permissions on the endpoint directory")
						epHealth.OK = false
					} else {
						report.AddFixed(epLabel, fmt.Sprintf("Created missing %s/ directory", sub))
					}
				}
			}

			// Verify SKILL.md files are parseable and spec-compliant
			for _, skillName := range []string{SkillSendable, SkillReadable} {
				skillDir := filepath.Join(epDir, "skills", skillName)
				skillPath := filepath.Join(skillDir, "SKILL.md")
				data, err := os.ReadFile(skillPath)
				if err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						continue // SKILL.md does not exist; skip
					}
					report.AddWarnWithHint(epLabel, fmt.Sprintf("Failed to read %s SKILL.md: %v", skillName, err),
						"check file permissions on the skills directory")
					continue
				}
				if _, err := ParseSkillFrontmatter(data); err != nil {
					report.AddWarnWithHint(epLabel, fmt.Sprintf("%s SKILL.md parse error: %v", skillName, err),
						"fix YAML frontmatter in SKILL.md")
				}
				// Run skills-ref spec compliance check (best-effort)
				if problems, err := ValidateSkillDir(skillDir); err != nil {
					report.AddWarnWithHint(epLabel, fmt.Sprintf("skills-ref validate %s: %v", skillName, err),
						"see Agent Skills spec for compliance requirements")
				} else if len(problems) > 0 {
					for _, p := range problems {
						report.AddWarnWithHint(epLabel, fmt.Sprintf("skills-ref: %s", p),
							"see Agent Skills spec for compliance requirements")
					}
				}
			}

			report.AddOK(epLabel, fmt.Sprintf("OK (produces: %v, consumes: %v)", ep.Produces, ep.Consumes))
			report.Endpoints = append(report.Endpoints, epHealth)
		}
	}

	// Check skills-ref toolchain
	checkSkillsRefToolchain(&report)

	// Check orphaned routes (per-repo to match routing scope)
	orphans := domain.DetectOrphansPerRepo(cfg)
	for _, kind := range orphans.UnconsumedKinds {
		report.AddWarnWithHint("", fmt.Sprintf("Orphaned: kind=%q is produced but not consumed", kind),
			`add a consumer for this kind or run "phonewave sync"`)
	}
	for _, kind := range orphans.UnproducedKinds {
		report.AddWarnWithHint("", fmt.Sprintf("Orphaned: kind=%q is consumed but not produced", kind),
			`add a producer for this kind or run "phonewave sync"`)
	}

	// Check resolved state file exists
	resolvedPath := filepath.Join(stateDir, ".run", domain.ResolvedStateFile)
	if _, err := os.Stat(resolvedPath); errors.Is(err, fs.ErrNotExist) {
		report.AddWarnWithHint("", "resolved.yaml not found: routes are being derived on-the-fly",
			`run "phonewave sync" to generate resolved state`)
	}

	// Success rate (informational)
	stats := ParseDeliveryStats(stateDir)
	m := domain.DeliveryMetrics{Delivered: stats.Delivered, Failed: stats.Failed}
	report.AddOK("success-rate", domain.FormatSuccessRate(m.SuccessRate(), stats.Delivered, stats.Delivered+stats.Failed))

	// Check daemon status
	report.DaemonStatus = checkDaemonStatus(stateDir)

	return report
}

// checkSkillsRefToolchain reports skills-ref and uv availability and venv state.
func checkSkillsRefToolchain(report *domain.DoctorReport) {
	venvDir := filepath.Join(os.TempDir(), domain.SkillsRefVenvName)

	// Check skills-ref on PATH
	if _, err := exec.LookPath("skills-ref"); err == nil {
		report.AddOK("skills-ref", "skills-ref found on PATH")
		return // Global install; no uv/venv needed
	}

	// Check uv on PATH
	_, uvErr := exec.LookPath("uv")
	if uvErr != nil {
		report.AddWarnWithHint("skills-ref",
			"uv not found on PATH: SKILL.md spec validation is unavailable",
			`install uv (https://docs.astral.sh/uv/) or "uv tool install skills-ref"`)
		return
	}

	// Check submodule availability
	subDir := findSkillsRefDir()
	if subDir == "" {
		report.AddWarnWithHint("skills-ref",
			"uv found but skills-ref submodule not available",
			`install globally: "uv tool install skills-ref"`)
		return
	}

	// uv + submodule available; report venv state
	if fi, err := os.Stat(venvDir); err == nil && fi.IsDir() {
		report.AddOK("skills-ref", fmt.Sprintf("uv + submodule ready (venv: %s)", venvDir))
	} else {
		report.AddOK("skills-ref", fmt.Sprintf("uv + submodule ready (venv will be created at %s on first use)", venvDir))
	}
}

func checkDaemonStatus(stateDir string) domain.DaemonHealthStatus {
	status := domain.DaemonHealthStatus{Checked: true}

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
