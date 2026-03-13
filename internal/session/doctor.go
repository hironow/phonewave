package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/hironow/phonewave/internal/domain"
)

// lookPathFn is injectable for testing.
var lookPathFn = exec.LookPath

// findSkillsRefDirFn is injectable for testing.
var findSkillsRefDirFn = findSkillsRefDir

// installSkillsRefFn runs "uv tool install skills-ref". Injectable for testing.
var installSkillsRefFn = func() error {
	cmd := exec.Command("uv", "tool", "install", "skills-ref")
	return cmd.Run()
}

// OverrideRepairInstallSkillsRef replaces the skills-ref installer for testing.
func OverrideRepairInstallSkillsRef(fn func() error) func() {
	old := installSkillsRefFn
	installSkillsRefFn = fn
	return func() { installSkillsRefFn = old }
}

// OverrideLookPath replaces exec.LookPath for testing.
func OverrideLookPath(fn func(string) (string, error)) func() {
	old := lookPathFn
	lookPathFn = fn
	return func() { lookPathFn = old }
}

// OverrideFindSkillsRefDir replaces findSkillsRefDir for testing.
func OverrideFindSkillsRefDir(fn func() string) func() {
	old := findSkillsRefDirFn
	findSkillsRefDirFn = fn
	return func() { findSkillsRefDirFn = old }
}

// repairSyncFn runs Sync(cfg) + WriteConfig to generate resolved state.
// Injectable for testing. Takes cfg and configPath (the actual config file path).
var repairSyncFn = func(cfg *domain.Config, configPath string) error {
	if _, err := Sync(cfg); err != nil {
		return err
	}
	return WriteConfig(configPath, cfg)
}

// OverrideRepairSync replaces the sync+write function for testing.
// The function receives (cfg, configPath) where configPath is the actual config file path.
func OverrideRepairSync(fn func(*domain.Config, string) error) func() {
	old := repairSyncFn
	repairSyncFn = fn
	return func() { repairSyncFn = old }
}

// FormatDoctorJSON marshals a DoctorReport to indented JSON.
func FormatDoctorJSON(report domain.DoctorReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// Doctor verifies ecosystem health and returns a report.
// configPath is the actual path to the config file (used by repair to write back).
// If empty, defaults to stateDir + ConfigFile.
func Doctor(cfg *domain.Config, stateDir string, repair bool, configPath string) domain.DoctorReport {
	if configPath == "" {
		configPath = filepath.Join(stateDir, domain.ConfigFile)
	}
	report := domain.DoctorReport{
		Healthy: true,
		DaemonStatus: domain.DaemonHealthStatus{
			Checked: true,
		},
	}

	// Check skills-ref toolchain BEFORE endpoint validation so repair
	// installs skills-ref before ValidateSkillDir runs.
	checkSkillsRefToolchain(&report, repair)

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
		if repair {
			if err := repairSyncFn(cfg, configPath); err != nil {
				report.AddWarnWithHint("", fmt.Sprintf("resolved.yaml repair failed: %v", err),
					`run "phonewave sync" manually`)
			} else {
				report.AddFixed("", "generated resolved.yaml via sync")
			}
		} else {
			report.AddWarnWithHint("", "resolved.yaml not found: routes are being derived on-the-fly",
				`run "phonewave doctor --repair" or "phonewave sync" to generate resolved state`)
		}
	}

	// Success rate (informational)
	stats := ParseDeliveryStats(stateDir)
	m := domain.DeliveryMetrics{Delivered: stats.Delivered, Failed: stats.Failed}
	report.AddOK("success-rate", domain.FormatSuccessRate(m.SuccessRate(), stats.Delivered, stats.Delivered+stats.Failed))

	// Check daemon status
	report.DaemonStatus = checkDaemonStatus(stateDir)

	// Repair: clean up stale PID file if daemon is not running
	if repair && !report.DaemonStatus.Running {
		pidPath := filepath.Join(stateDir, "watch.pid")
		if _, err := os.Stat(pidPath); err == nil {
			os.Remove(pidPath)
		}
	}

	return report
}

// checkSkillsRefToolchain reports skills-ref and uv availability and venv state.
func checkSkillsRefToolchain(report *domain.DoctorReport, repair bool) {
	// Check skills-ref on PATH (global install)
	if _, err := lookPathFn("skills-ref"); err == nil {
		report.AddOK("skills-ref", "skills-ref found on PATH")
		return
	}

	// Check uv on PATH
	_, uvErr := lookPathFn("uv")
	if uvErr != nil {
		report.AddWarnWithHint("skills-ref",
			"uv not found on PATH: SKILL.md spec validation is unavailable",
			`install uv (https://docs.astral.sh/uv/) or "uv tool install skills-ref"`)
		return
	}

	// Check submodule availability FIRST (idempotent — no side effects)
	subDir := findSkillsRefDirFn()
	if subDir != "" {
		venvDir := filepath.Join(os.TempDir(), domain.SkillsRefVenvName)
		if fi, err := os.Stat(venvDir); err == nil && fi.IsDir() {
			report.AddOK("skills-ref", fmt.Sprintf("uv + submodule ready (venv: %s)", venvDir))
		} else {
			report.AddOK("skills-ref", fmt.Sprintf("uv + submodule ready (venv will be created at %s on first use)", venvDir))
		}
		return
	}

	// No submodule, no global install — attempt repair if requested
	if repair {
		if err := installSkillsRefFn(); err != nil {
			report.AddWarnWithHint("skills-ref",
				fmt.Sprintf("uv tool install skills-ref failed: %v", err),
				`try manually: "uv tool install skills-ref"`)
		} else {
			// Verify the binary is actually on PATH after install
			if _, err := lookPathFn("skills-ref"); err != nil {
				report.AddWarnWithHint("skills-ref",
					"uv tool install skills-ref succeeded but skills-ref is not on PATH",
					`add uv's tool bin directory to your PATH (e.g. ~/.local/bin)`)
			} else {
				report.AddFixed("skills-ref", "installed skills-ref via uv tool install")
			}
		}
		return
	}

	report.AddWarnWithHint("skills-ref",
		"uv found but skills-ref not installed",
		`run "phonewave doctor --repair" or "uv tool install skills-ref"`)
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

	if IsProcessAlive(pid) {
		status.Running = true
		status.PID = pid
	}

	return status
}

// IsProcessAlive checks whether a process with the given PID is running.
// On Unix, it sends signal 0 to verify liveness. On Windows, it uses
// FindProcess which succeeds if the process exists.
func IsProcessAlive(pid int) bool {
	if runtime.GOOS == "windows" {
		// On Windows, FindProcess succeeds only if the process exists.
		p, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		// On Windows, FindProcess does not fail for non-existent PIDs in Go,
		// but we can attempt to open the process handle via Signal to verify.
		// However, Signal(0) is not supported on Windows, so we treat
		// FindProcess success as "alive" (best-effort).
		_ = p
		return true
	}

	// Unix: FindProcess always succeeds; send signal 0 to check.
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		// EPERM means process exists but we lack permission to signal it
		return errors.Is(err, syscall.EPERM)
	}

	return true
}
