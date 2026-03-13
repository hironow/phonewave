# doctor --repair Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--repair` flag to `phonewave doctor` that auto-fixes repairable WARN items (skills-ref install, resolved.yaml generation, stale PID cleanup).

**Architecture:** Extend `Doctor()` with a `repair bool` parameter. When repair=true, each repairable check attempts remediation and reports "fixed" instead of "warn". Repair actions are injectable for testing. `checkDaemonStatus` signature is NOT changed — stale PID cleanup is done in `Doctor()` after the status check.

**Tech Stack:** Go, cobra, exec.Command (for `uv tool install skills-ref`)

---

### Task 1: Add repair parameter to Doctor signature + stale PID cleanup

**Files:**
- Modify: `internal/session/doctor.go` — `Doctor(cfg, stateDir, repair)` signature
- Modify: `internal/cmd/doctor.go` — pass repair flag
- Modify: `internal/session/doctor_test.go` — update all existing `Doctor()` call sites

**Step 1: Write the failing test**

Add to `internal/session/doctor_test.go`:

```go
func TestDoctor_RepairStalePID(t *testing.T) {
	// given — stale PID file with non-existent process
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	pidPath := filepath.Join(stateDir, "watch.pid")
	os.WriteFile(pidPath, []byte("999999999"), 0644)

	cfg := &domain.Config{}

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true)

	// then — stale PID file should be removed
	if _, err := os.Stat(pidPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("expected stale PID file to be removed")
	}
	// daemon status should reflect cleanup
	if report.DaemonStatus.Running {
		t.Error("daemon should not be running after stale PID cleanup")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/nino/tap/phonewave && go test ./internal/session/ -run TestDoctor_RepairStalePID -count=1 -v ; cd -`
Expected: FAIL — `Doctor()` takes 2 args, not 3

**Step 3: Update Doctor signature and fix all call sites**

In `internal/session/doctor.go`:
```go
func Doctor(cfg *domain.Config, stateDir string, repair bool) domain.DoctorReport {
```

In `internal/cmd/doctor.go`, add `--repair` flag (after the `cmd` definition, before `return cmd`):
```go
cmd.Flags().Bool("repair", false, "Auto-fix repairable issues")
```

And in RunE:
```go
repair, _ := cmd.Flags().GetBool("repair")
report := session.Doctor(cfg, stateDir, repair)
```

Update all existing test call sites from `session.Doctor(cfg, stateDir)` to `session.Doctor(cfg, stateDir, false)`.

**Step 4: Implement stale PID repair in Doctor() (NOT in checkDaemonStatus)**

IMPORTANT: Do NOT change `checkDaemonStatus` signature — it is also called from `status.go:93`.
Instead, add stale PID cleanup in `Doctor()` after the `checkDaemonStatus` call:

```go
// Check daemon status
report.DaemonStatus = checkDaemonStatus(stateDir)

// Repair: clean up stale PID file if daemon is not running
if repair && !report.DaemonStatus.Running {
	pidPath := filepath.Join(stateDir, "watch.pid")
	if _, err := os.Stat(pidPath); err == nil {
		os.Remove(pidPath)
	}
}
```

**Step 5: Run tests**

Run: `cd /Users/nino/tap/phonewave && go test ./internal/session/ -run TestDoctor -count=1 -v ; cd -`
Expected: ALL PASS

**Step 6: Commit**

```bash
cd /Users/nino/tap/phonewave && git add internal/session/doctor.go internal/session/doctor_test.go internal/cmd/doctor.go && git commit -m "feat(doctor): add --repair flag with stale PID cleanup" ; cd -
```

---

### Task 2: Repair skills-ref (uv tool install)

**Files:**
- Modify: `internal/session/doctor.go` — `checkSkillsRefToolchain` with repair
- Modify: `internal/session/doctor_test.go` — new test

**Step 1: Write the failing test**

```go
func TestDoctor_RepairSkillsRef_UvAvailable(t *testing.T) {
	// given — uv is on PATH but skills-ref is not, and no submodule
	cfg := &domain.Config{}
	stateDir := filepath.Join(t.TempDir(), domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	// Inject: simulate uv available, skills-ref not available
	var installCalled bool
	cleanup := session.OverrideRepairInstallSkillsRef(func() error {
		installCalled = true
		return nil
	})
	defer cleanup()

	cleanup2 := session.OverrideLookPath(func(name string) (string, error) {
		if name == "uv" {
			return "/usr/bin/uv", nil
		}
		return "", exec.ErrNotFound
	})
	defer cleanup2()

	cleanup3 := session.OverrideFindSkillsRefDir(func() string {
		return "" // no submodule
	})
	defer cleanup3()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true)

	// then — install should have been called
	if !installCalled {
		t.Error("expected uv tool install skills-ref to be called")
	}
	// Should have a "fixed" issue
	hasFixed := false
	for _, issue := range report.Issues {
		if issue.Endpoint == "skills-ref" && issue.Severity == "fixed" {
			hasFixed = true
		}
	}
	if !hasFixed {
		t.Errorf("expected fixed issue for skills-ref, got: %v", report.Issues)
	}
}

func TestDoctor_RepairSkillsRef_SubmoduleAvailable_NoInstall(t *testing.T) {
	// given — uv on PATH, skills-ref NOT on PATH, but submodule IS available
	cfg := &domain.Config{}
	stateDir := filepath.Join(t.TempDir(), domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	var installCalled bool
	cleanup := session.OverrideRepairInstallSkillsRef(func() error {
		installCalled = true
		return nil
	})
	defer cleanup()

	cleanup2 := session.OverrideLookPath(func(name string) (string, error) {
		if name == "uv" {
			return "/usr/bin/uv", nil
		}
		return "", exec.ErrNotFound
	})
	defer cleanup2()

	cleanup3 := session.OverrideFindSkillsRefDir(func() string {
		return "/some/submodule/path" // submodule available
	})
	defer cleanup3()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true)

	// then — install should NOT have been called (submodule suffices)
	if installCalled {
		t.Error("should not install skills-ref when submodule is available")
	}
	// Should have an OK issue, not fixed
	hasOK := false
	for _, issue := range report.Issues {
		if issue.Endpoint == "skills-ref" && issue.Severity == "ok" {
			hasOK = true
		}
	}
	if !hasOK {
		t.Errorf("expected OK issue for skills-ref with submodule, got: %v", report.Issues)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/nino/tap/phonewave && go test ./internal/session/ -run TestDoctor_RepairSkillsRef -count=1 -v ; cd -`
Expected: FAIL — `OverrideRepairInstallSkillsRef` not defined

**Step 3: Implement injectable repair functions**

In `internal/session/doctor.go`, add injectable functions:

```go
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
```

Update `checkSkillsRefToolchain` to accept `repair bool`. Key: check submodule FIRST, only install if no submodule (idempotent):

```go
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
		// Submodule available — no install needed regardless of repair flag
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
			report.AddFixed("skills-ref", "installed skills-ref via uv tool install")
		}
		return
	}

	report.AddWarnWithHint("skills-ref",
		"uv found but skills-ref not installed",
		`run "phonewave doctor --repair" or "uv tool install skills-ref"`)
}
```

Also update `checkSkillsRefToolchain` call sites in `Doctor()`:
```go
checkSkillsRefToolchain(&report, repair)
```

**Step 4: Run tests**

Run: `cd /Users/nino/tap/phonewave && go test ./internal/session/ -run TestDoctor -count=1 -v ; cd -`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/nino/tap/phonewave && git add internal/session/doctor.go internal/session/doctor_test.go && git commit -m "feat(doctor): --repair installs skills-ref via uv tool (idempotent)" ; cd -
```

---

### Task 3: Repair resolved.yaml (Sync + WriteConfig)

**Files:**
- Modify: `internal/session/doctor.go` — resolved.yaml repair section
- Modify: `internal/session/doctor_test.go` — new test

**Step 1: Write the failing test**

```go
func TestDoctor_RepairResolvedState(t *testing.T) {
	// given — resolved.yaml does not exist
	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, domain.StateDir)
	runDir := filepath.Join(stateDir, ".run")
	os.MkdirAll(runDir, 0755)
	os.MkdirAll(filepath.Join(repoDir, ".siren", "outbox"), 0755)
	os.MkdirAll(filepath.Join(repoDir, ".siren", "inbox"), 0755)

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"spec"}},
				},
			},
		},
	}

	// Inject repair sync+write function
	var repairCalled bool
	cleanup := session.OverrideRepairSync(func(c *domain.Config, sd string) error {
		repairCalled = true
		// Simulate: Sync updates cfg, then WriteConfig writes resolved.yaml
		return os.WriteFile(
			filepath.Join(sd, ".run", domain.ResolvedStateFile),
			[]byte("routes: []\n"), 0644)
	})
	defer cleanup()

	// when — repair=true
	report := session.Doctor(cfg, stateDir, true)

	// then
	if !repairCalled {
		t.Error("expected repair sync to be called for missing resolved.yaml")
	}
	hasFixed := false
	for _, issue := range report.Issues {
		if issue.Severity == "fixed" && strings.Contains(issue.Message, "resolved") {
			hasFixed = true
		}
	}
	if !hasFixed {
		t.Errorf("expected fixed issue for resolved.yaml, got: %v", report.Issues)
	}
}
```

**Step 2: Implement**

In `internal/session/doctor.go`, add injectable sync+write:

```go
// repairSyncFn runs Sync(cfg) + WriteConfig to generate resolved state.
// Injectable for testing. Takes cfg and stateDir.
var repairSyncFn = func(cfg *domain.Config, stateDir string) error {
	if _, err := Sync(cfg); err != nil {
		return err
	}
	configPath := filepath.Join(stateDir, domain.ConfigFile)
	return WriteConfig(configPath, cfg)
}

// OverrideRepairSync replaces the sync+write function for testing.
func OverrideRepairSync(fn func(*domain.Config, string) error) func() {
	old := repairSyncFn
	repairSyncFn = fn
	return func() { repairSyncFn = old }
}
```

Update the resolved.yaml check in `Doctor()`:

```go
// Check resolved state file exists
resolvedPath := filepath.Join(stateDir, ".run", domain.ResolvedStateFile)
if _, err := os.Stat(resolvedPath); errors.Is(err, fs.ErrNotExist) {
	if repair {
		if err := repairSyncFn(cfg, stateDir); err != nil {
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
```

**Step 3: Run tests**

Run: `cd /Users/nino/tap/phonewave && go test ./internal/session/ -run TestDoctor -count=1 -v ; cd -`
Expected: ALL PASS

**Step 4: Commit**

```bash
cd /Users/nino/tap/phonewave && git add internal/session/doctor.go internal/session/doctor_test.go && git commit -m "feat(doctor): --repair generates resolved.yaml via sync+write" ; cd -
```

---

### Task 4: Update hints and docgen

**Files:**
- Modify: `internal/session/doctor.go` — update existing hints to mention `--repair`
- Run: `just docgen` (if available)

**Step 1: Update hints**

In all WARN hints that have a manual repair equivalent, add `--repair` reference:
- skills-ref: `run "phonewave doctor --repair" or "uv tool install skills-ref"`
- resolved.yaml: `run "phonewave doctor --repair" or "phonewave sync"`

**Step 2: Run full test suite**

Run: `cd /Users/nino/tap/phonewave && go test ./... -count=1 ; cd -`
Expected: ALL PASS

**Step 3: Run docgen if available**

Run: `cd /Users/nino/tap/phonewave && just docgen 2>/dev/null || echo "no docgen" ; cd -`

**Step 4: Commit**

```bash
cd /Users/nino/tap/phonewave && git add -A && git commit -m "docs: update doctor hints to reference --repair" ; cd -
```
