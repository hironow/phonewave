package session

// white-box-reason: session internals: tests unexported multi-tool repository setup helpers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

// setupTestRepo creates a temporary repository with the given tool endpoints.
func setupTestRepo(t *testing.T, tools map[string]struct{ produces, consumes []string }) string {
	t.Helper()
	repoDir := t.TempDir()

	for dotDir, caps := range tools {
		if len(caps.produces) > 0 {
			dir := filepath.Join(repoDir, dotDir, "skills", "dmail-sendable")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			content := "---\nname: dmail-sendable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n"
			for _, k := range caps.produces {
				content += "    - kind: " + k + "\n"
			}
			content += "---\n"
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if len(caps.consumes) > 0 {
			dir := filepath.Join(repoDir, dotDir, "skills", "dmail-readable")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			content := "---\nname: dmail-readable\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  consumes:\n"
			for _, k := range caps.consumes {
				content += "    - kind: " + k + "\n"
			}
			content += "---\n"
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	return repoDir
}

func TestInit_FullEcosystem(t *testing.T) {
	// given — a repo with all three tools
	repo := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren":      {produces: []string{"specification"}, consumes: []string{"design-feedback"}},
		".expedition": {produces: []string{"report"}, consumes: []string{"specification", "design-feedback"}},
		".gate":       {produces: []string{"design-feedback"}, consumes: []string{"report"}},
	})

	// when
	result, err := Init(context.Background(), []string{repo})

	// then
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if result.RepoCount != 1 {
		t.Errorf("RepoCount = %d, want 1", result.RepoCount)
	}
	if len(result.Config.Repositories) != 1 {
		t.Fatalf("repositories = %d, want 1", len(result.Config.Repositories))
	}
	if len(result.Config.Repositories[0].Endpoints) != 3 {
		t.Errorf("endpoints = %d, want 3", len(result.Config.Repositories[0].Endpoints))
	}
	if len(result.Config.Routes) != 3 {
		t.Errorf("routes = %d, want 3", len(result.Config.Routes))
	}
	if len(result.Orphans.UnconsumedKinds) != 0 {
		t.Errorf("unconsumed = %v, want none", result.Orphans.UnconsumedKinds)
	}
}

func TestAdd_NewRepository(t *testing.T) {
	// given — existing config with one repo
	repo1 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren": {produces: []string{"specification"}, consumes: []string{"design-feedback"}},
	})
	result, err := Init(context.Background(), []string{repo1})
	if err != nil {
		t.Fatal(err)
	}

	// Set up a second repo
	repo2 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".expedition": {produces: []string{"report"}, consumes: []string{"specification"}},
	})

	// when
	addResult, err := Add(context.Background(), result.Config, repo2)

	// then
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(result.Config.Repositories) != 2 {
		t.Errorf("repositories = %d, want 2", len(result.Config.Repositories))
	}
	_ = addResult.Orphans
}

func TestAdd_DuplicateRepository(t *testing.T) {
	repo := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren": {produces: []string{"specification"}},
	})
	result, err := Init(context.Background(), []string{repo})
	if err != nil {
		t.Fatal(err)
	}

	// when — add the same repo again
	_, err = Add(context.Background(), result.Config, repo)
	if err == nil {
		t.Fatal("expected error for duplicate repository")
	}
}

func TestAdd_SkillsRefWarnings(t *testing.T) {
	if !skillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — existing config, new repo with non-compliant SKILL.md
	repo1 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".gate": {produces: []string{"design-feedback"}},
	})
	initResult, err := Init(context.Background(), []string{repo1})
	if err != nil {
		t.Fatal(err)
	}

	// Non-compliant repo: name doesn't match directory
	repo2 := t.TempDir()
	sendableDir := filepath.Join(repo2, ".siren", "skills", "dmail-sendable")
	if err := os.MkdirAll(sendableDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeSkillFile(t, filepath.Join(sendableDir, "SKILL.md"),
		"---\nname: wrong-name\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: specification\n---\n")

	// when
	addResult, err := Add(context.Background(), initResult.Config, repo2)

	// then
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	hasSkillsRefWarn := false
	for _, w := range addResult.Warnings {
		if strings.Contains(w, "skills-ref") {
			hasSkillsRefWarn = true
		}
	}
	if !hasSkillsRefWarn {
		t.Errorf("expected skills-ref warning from Add, got warnings: %v", addResult.Warnings)
	}
}

func TestRemove_ExistingRepository(t *testing.T) {
	repo1 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren": {produces: []string{"specification"}},
	})
	repo2 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".expedition": {produces: []string{"report"}},
	})

	result, err := Init(context.Background(), []string{repo1, repo2})
	if err != nil {
		t.Fatal(err)
	}

	// when
	_, err = Remove(result.Config, repo1)

	// then
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(result.Config.Repositories) != 1 {
		t.Errorf("repositories = %d, want 1", len(result.Config.Repositories))
	}
}

func TestDiffEndpoints_DetectsAdded(t *testing.T) {
	// given
	old := map[string]domain.EndpointConfig{}
	new_ := map[string]domain.EndpointConfig{
		"repo-a/.siren": {Dir: ".siren", Produces: []string{"specification"}},
	}

	// when
	diffs := diffEndpoints(old, new_)

	// then
	if len(diffs) != 1 {
		t.Fatalf("diffs = %d, want 1", len(diffs))
	}
	if diffs[0].Change != "added" {
		t.Errorf("change = %q, want added", diffs[0].Change)
	}
}

func TestDiffEndpoints_DetectsRemoved(t *testing.T) {
	// given
	old := map[string]domain.EndpointConfig{
		"repo-a/.siren": {Dir: ".siren", Produces: []string{"specification"}},
	}
	new_ := map[string]domain.EndpointConfig{}

	// when
	diffs := diffEndpoints(old, new_)

	// then
	if len(diffs) != 1 {
		t.Fatalf("diffs = %d, want 1", len(diffs))
	}
	if diffs[0].Change != "removed" {
		t.Errorf("change = %q, want removed", diffs[0].Change)
	}
}

func TestDiffEndpoints_DetectsChanged(t *testing.T) {
	// given
	old := map[string]domain.EndpointConfig{
		"repo-a/.expedition": {Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
	}
	new_ := map[string]domain.EndpointConfig{
		"repo-a/.expedition": {Dir: ".expedition", Produces: []string{"report", "analysis"}, Consumes: []string{"specification"}},
	}

	// when
	diffs := diffEndpoints(old, new_)

	// then
	if len(diffs) != 1 {
		t.Fatalf("diffs = %d, want 1", len(diffs))
	}
	if diffs[0].Change != "changed" {
		t.Errorf("change = %q, want changed", diffs[0].Change)
	}
}

func TestDiffRoutes_DetectsAddedAndRemoved(t *testing.T) {
	// given
	old := map[string]domain.RouteConfig{
		"specification:.siren/outbox": {Kind: "specification", From: ".siren/outbox"},
	}
	new_ := map[string]domain.RouteConfig{
		"report:.expedition/outbox": {Kind: "report", From: ".expedition/outbox"},
	}

	// when
	diffs := diffRoutes(old, new_)

	// then
	if len(diffs) != 2 {
		t.Fatalf("diffs = %d, want 2 (1 added, 1 removed)", len(diffs))
	}

	var added, removed int
	for _, d := range diffs {
		switch d.Change {
		case "added":
			added++
		case "removed":
			removed++
		}
	}
	if added != 1 || removed != 1 {
		t.Errorf("added=%d removed=%d, want 1 each", added, removed)
	}
}

func TestInit_SkillsRefWarnings(t *testing.T) {
	if !skillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — SKILL.md with name not matching directory (Agent Skills spec violation)
	repoDir := t.TempDir()
	sendableDir := filepath.Join(repoDir, ".siren", "skills", "dmail-sendable")
	if err := os.MkdirAll(sendableDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeSkillFile(t, filepath.Join(sendableDir, "SKILL.md"),
		"---\nname: wrong-name\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: specification\n---\n")

	// when
	result, err := Init(context.Background(), []string{repoDir})

	// then
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	hasSkillsRefWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "skills-ref") {
			hasSkillsRefWarn = true
		}
	}
	if !hasSkillsRefWarn {
		t.Errorf("expected skills-ref validation warning in Init result, got warnings: %v", result.Warnings)
	}
}

func TestSync_UpdatesEndpoints(t *testing.T) {
	repo := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren": {produces: []string{"specification"}, consumes: []string{"design-feedback"}},
	})

	result, err := Init(context.Background(), []string{repo})
	if err != nil {
		t.Fatal(err)
	}

	// when — sync (even if nothing changed, should still work)
	report, err := Sync(context.Background(), result.Config)

	// then
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(result.Config.Repositories) != 1 {
		t.Errorf("repositories = %d, want 1", len(result.Config.Repositories))
	}
	if report.RepoCount != 1 {
		t.Errorf("report.RepoCount = %d, want 1", report.RepoCount)
	}
	// No changes expected since nothing changed on disk
	if len(report.EndpointChanges) != 0 {
		t.Errorf("endpoint changes = %d, want 0", len(report.EndpointChanges))
	}
	if len(report.RouteChanges) != 0 {
		t.Errorf("route changes = %d, want 0", len(report.RouteChanges))
	}
}

func TestSync_SkillsRefWarnings(t *testing.T) {
	if !skillsRefAvailable() {
		t.Skip("skills-ref not available")
	}

	// given — repo with non-compliant SKILL.md (name doesn't match directory)
	repoDir := t.TempDir()
	sendableDir := filepath.Join(repoDir, ".siren", "skills", "dmail-sendable")
	if err := os.MkdirAll(sendableDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeSkillFile(t, filepath.Join(sendableDir, "SKILL.md"),
		"---\nname: wrong-name\ndescription: test\nlicense: Apache-2.0\nmetadata:\n  dmail-schema-version: \"1\"\n  produces:\n    - kind: specification\n---\n")

	result, err := Init(context.Background(), []string{repoDir})
	if err != nil {
		t.Fatal(err)
	}

	// when
	report, err := Sync(context.Background(), result.Config)

	// then
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	hasSkillsRefWarn := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "skills-ref") {
			hasSkillsRefWarn = true
		}
	}
	if !hasSkillsRefWarn {
		t.Errorf("expected skills-ref warning from Sync, got warnings: %v", report.Warnings)
	}
}

func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
