package phonewave

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// skillsRefTimeout is the maximum time allowed for a single skills-ref invocation.
const skillsRefTimeout = 30 * time.Second

// ValidateSkillDir runs skills-ref validate against a skill directory
// and returns any validation problems found.
// Returns nil problems if skills-ref is not available (best-effort).
func ValidateSkillDir(skillDir string) ([]string, error) {
	cmd, err := skillsRefCommand(skillDir)
	if err != nil {
		// skills-ref not available; skip validation
		return nil, nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Exit code 1 = validation errors found (not a command failure)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return parseValidationOutput(string(output)), nil
		}
		return nil, fmt.Errorf("skills-ref validate: %w: %s", err, string(output))
	}

	return nil, nil
}

// skillsRefCommand builds the exec.Cmd for skills-ref validate.
// Discovery order:
//  1. "skills-ref" on PATH (global install)
//  2. "uv run --project <submodule>" (bundled submodule)
func skillsRefCommand(skillDir string) (*exec.Cmd, error) {
	ctx, cancel := context.WithTimeout(context.Background(), skillsRefTimeout)
	_ = cancel // caller owns cmd lifetime; cancel is available via cmd.Cancel

	if path, err := exec.LookPath("skills-ref"); err == nil {
		cmd := exec.CommandContext(ctx, path, "validate", skillDir)
		cmd.Cancel = func() error { cancel(); return cmd.Process.Kill() }
		return cmd, nil
	}
	if uvPath, err := exec.LookPath("uv"); err == nil {
		if subDir := findSkillsRefDir(); subDir != "" {
			cmd := exec.CommandContext(ctx, uvPath, "run", "--project", subDir, "skills-ref", "validate", skillDir)
			cmd.Cancel = func() error { cancel(); return cmd.Process.Kill() }
			return cmd, nil
		}
	}
	cancel()
	return nil, fmt.Errorf("skills-ref not found (install via 'uv tool install skills-ref' or ensure submodule is present)")
}

// findSkillsRefDir locates the bundled skills-ref submodule.
// Discovery order:
//  1. PHONEWAVE_SKILLS_REF environment variable (explicit override)
//  2. Walk up from the executable path (handles installed-in-repo binaries)
//  3. Walk up from CWD (handles development and test runs)
func findSkillsRefDir() string {
	// 1. Explicit override
	if dir := os.Getenv("PHONEWAVE_SKILLS_REF"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
			return dir
		}
	}

	// 2. Walk up from executable path
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		if dir := walkUpForSkillsRef(filepath.Dir(exe)); dir != "" {
			return dir
		}
	}

	// 3. Walk up from CWD
	if cwd, err := os.Getwd(); err == nil {
		if dir := walkUpForSkillsRef(cwd); dir != "" {
			return dir
		}
	}

	return ""
}

// walkUpForSkillsRef walks up directory ancestors from startDir looking
// for skills-ref/skills-ref/pyproject.toml.
func walkUpForSkillsRef(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, "skills-ref", "skills-ref", "pyproject.toml")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Join(dir, "skills-ref", "skills-ref")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// validateEndpointSkills runs skills-ref validation on an endpoint's skill directories.
// Validates any skill directory that exists on disk, regardless of whether
// the endpoint config declares produces/consumes (handles migration scenarios).
func validateEndpointSkills(repoPath string, ep EndpointConfig) []string {
	var warnings []string
	epLabel := filepath.Base(repoPath) + "/" + ep.Dir

	for _, skillName := range []string{"dmail-sendable", "dmail-readable"} {
		skillDir := filepath.Join(repoPath, ep.Dir, "skills", skillName)
		if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
			if os.IsNotExist(err) {
				continue // SKILL.md does not exist on disk; nothing to validate
			}
			warnings = append(warnings, fmt.Sprintf("skills-ref: cannot access %s/%s SKILL.md: %v", epLabel, skillName, err))
			continue
		}
		if problems, err := ValidateSkillDir(skillDir); err != nil {
			warnings = append(warnings, fmt.Sprintf("skills-ref validate %s/%s: %v", epLabel, skillName, err))
		} else if len(problems) > 0 {
			for _, p := range problems {
				warnings = append(warnings, fmt.Sprintf("skills-ref: %s/%s: %s", epLabel, skillName, p))
			}
		}
	}
	return warnings
}

// collectSkillWarnings runs skills-ref validation across repositories in cfg.
// If filterRepoPath is non-empty, only that repository's endpoints are checked.
func collectSkillWarnings(cfg *Config, filterRepoPath string) []string {
	var warnings []string
	for _, repo := range cfg.Repositories {
		if filterRepoPath != "" && repo.Path != filterRepoPath {
			continue
		}
		for _, ep := range repo.Endpoints {
			warnings = append(warnings, validateEndpointSkills(repo.Path, ep)...)
		}
	}
	return warnings
}

// parseValidationOutput extracts problem messages from skills-ref output.
func parseValidationOutput(output string) []string {
	var problems []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			problems = append(problems, strings.TrimPrefix(line, "- "))
		}
	}
	return problems
}
