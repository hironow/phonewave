package session

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	pond "github.com/alitto/pond/v2"
	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
)

// skillsRefTimeout is the maximum time allowed for a single skills-ref invocation.
const skillsRefTimeout = 30 * time.Second

// ValidateSkillDir runs skills-ref validate against a skill directory
// and returns any validation problems found.
// Returns nil problems if skills-ref is not available (best-effort).
func ValidateSkillDir(skillDir string) ([]string, error) {
	cmd, cancel, err := skillsRefCommand(skillDir)
	if err != nil {
		// skills-ref not available; skip validation
		return nil, nil
	}
	defer cancel()

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
// Returns the command and a cancel function that the caller must invoke
// after command completion to release the timeout context.
// Discovery order:
//  1. "skills-ref" on PATH (global install)
//  2. "uv run --project <submodule>" (bundled submodule)
func skillsRefCommand(skillDir string) (*exec.Cmd, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), skillsRefTimeout)

	if path, err := exec.LookPath("skills-ref"); err == nil {
		cmd := exec.CommandContext(ctx, path, "validate", skillDir)
		cmd.Cancel = func() error { cancel(); return cmd.Process.Kill() }
		return cmd, cancel, nil
	}
	if uvPath, err := exec.LookPath("uv"); err == nil {
		if subDir := findSkillsRefDir(); subDir != "" {
			cmd := exec.CommandContext(ctx, uvPath, "run", "--project", subDir, "skills-ref", "validate", skillDir)
			// Redirect uv's venv to a temp directory so Python artifacts
			// don't pollute the Go repository tree.
			// NOTE: os.TempDir() is used instead of os.UserCacheDir() for
			// simplicity and cross-platform consistency. The tradeoff is that
			// temp dirs may be cleaned by the OS between reboots, causing a
			// venv rebuild. If isolation becomes a concern, consider switching
			// to os.UserCacheDir() here and in doctor.go / status.go.
			venvDir := filepath.Join(os.TempDir(), domain.SkillsRefVenvName)
			cmd.Env = append(os.Environ(), "UV_PROJECT_ENVIRONMENT="+venvDir)
			cmd.Cancel = func() error { cancel(); return cmd.Process.Kill() }
			return cmd, cancel, nil
		}
	}
	cancel()
	return nil, nil, fmt.Errorf("skills-ref not found (install via 'uv tool install skills-ref' or ensure submodule is present)")
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
// the endpoint config declares produces/consumes.
func validateEndpointSkills(repoPath string, ep domain.EndpointConfig) []string {
	var warnings []string
	epLabel := filepath.Base(repoPath) + "/" + ep.Dir // nosemgrep: adr0005-string-concat-file-path — display label, not file path [permanent]

	for _, skillName := range []string{SkillSendable, SkillReadable} {
		skillDir := filepath.Join(repoPath, ep.Dir, "skills", skillName)
		if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
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

// validationTarget pairs a repo path with an endpoint for concurrent validation.
type validationTarget struct {
	repoPath string
	ep       domain.EndpointConfig
}

// collectSkillWarnings runs skills-ref validation concurrently across
// repositories in cfg. Each endpoint is validated in a separate worker.
// If filterRepoPath is non-empty, only that repository's endpoints are checked.
func collectSkillWarnings(ctx context.Context, cfg *domain.Config, filterRepoPath string) []string {
	ctx, span := platform.Tracer.Start(ctx, "phonewave.validate")
	defer span.End()
	_ = ctx // span-only; no child spans needed
	start := time.Now()

	var targets []validationTarget
	for _, repo := range cfg.Repositories {
		if filterRepoPath != "" && repo.Path != filterRepoPath {
			continue
		}
		for _, ep := range repo.Endpoints {
			targets = append(targets, validationTarget{repoPath: repo.Path, ep: ep})
		}
	}

	if len(targets) == 0 {
		span.SetAttributes(attribute.Int("skills_ref.problem.count", 0))
		if platform.IsDetailDebug() {
			span.SetAttributes(attribute.Int64("skills_ref.exec_ms", time.Since(start).Milliseconds()))
		}
		return nil
	}

	pool := pond.NewResultPool[[]string](runtime.NumCPU())
	group := pool.NewGroup()

	for _, t := range targets {
		t := t // capture for goroutine
		group.Submit(func() []string {
			return validateEndpointSkills(t.repoPath, t.ep)
		})
	}

	// ResultTaskGroup.Wait() preserves submission order.
	results, _ := group.Wait()
	pool.StopAndWait()

	var warnings []string
	for _, ws := range results {
		warnings = append(warnings, ws...)
	}

	span.SetAttributes(attribute.Int("skills_ref.problem.count", len(warnings)))
	if platform.IsDetailDebug() {
		span.SetAttributes(attribute.Int64("skills_ref.exec_ms", time.Since(start).Milliseconds()))
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
