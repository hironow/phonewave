package phonewave

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
//  2. "uv run --directory <submodule>" (bundled submodule)
func skillsRefCommand(skillDir string) (*exec.Cmd, error) {
	if path, err := exec.LookPath("skills-ref"); err == nil {
		return exec.Command(path, "validate", skillDir), nil
	}
	if uvPath, err := exec.LookPath("uv"); err == nil {
		if subDir := findSkillsRefDir(); subDir != "" {
			return exec.Command(uvPath, "run", "--directory", subDir, "skills-ref", "validate", skillDir), nil
		}
	}
	return nil, fmt.Errorf("skills-ref not found (install via 'uv tool install skills-ref' or ensure submodule is present)")
}

// findSkillsRefDir walks up from CWD looking for the bundled
// skills-ref submodule (skills-ref/skills-ref/pyproject.toml).
func findSkillsRefDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
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
func validateEndpointSkills(repoPath string, ep EndpointConfig) []string {
	var warnings []string
	epLabel := filepath.Base(repoPath) + "/" + ep.Dir

	for _, skill := range []struct {
		name    string
		hasDecl bool
	}{
		{"dmail-sendable", len(ep.Produces) > 0},
		{"dmail-readable", len(ep.Consumes) > 0},
	} {
		if !skill.hasDecl {
			continue
		}
		skillDir := filepath.Join(repoPath, ep.Dir, "skills", skill.name)
		if problems, err := ValidateSkillDir(skillDir); err != nil {
			warnings = append(warnings, fmt.Sprintf("skills-ref validate %s/%s: %v", epLabel, skill.name, err))
		} else if len(problems) > 0 {
			for _, p := range problems {
				warnings = append(warnings, fmt.Sprintf("skills-ref: %s/%s: %s", epLabel, skill.name, p))
			}
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
