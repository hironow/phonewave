package phonewave

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary repository with the given tool endpoints.
func setupTestRepo(t *testing.T, tools map[string]struct{ produces, consumes []string }) string {
	t.Helper()
	repoDir := t.TempDir()

	for dotDir, caps := range tools {
		for _, kind := range caps.produces {
			dir := filepath.Join(repoDir, dotDir, "skills", "dmail-sendable")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			content := "---\nname: dmail-sendable\nproduces:\n"
			for _, k := range caps.produces {
				content += "  - kind: " + k + "\n"
			}
			content += "---\n"
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			_ = kind
			break // write once for all produces
		}
		for _, kind := range caps.consumes {
			dir := filepath.Join(repoDir, dotDir, "skills", "dmail-readable")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			content := "---\nname: dmail-readable\nconsumes:\n"
			for _, k := range caps.consumes {
				content += "  - kind: " + k + "\n"
			}
			content += "---\n"
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			_ = kind
			break // write once for all consumes
		}
	}

	return repoDir
}

func TestInit_FullEcosystem(t *testing.T) {
	// given — a repo with all three tools
	repo := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren":      {produces: []string{"specification"}, consumes: []string{"feedback"}},
		".expedition": {produces: []string{"report"}, consumes: []string{"specification", "feedback"}},
		".divergence": {produces: []string{"feedback"}, consumes: []string{"report"}},
	})

	// when
	result, err := Init([]string{repo})

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
		".siren": {produces: []string{"specification"}, consumes: []string{"feedback"}},
	})
	result, err := Init([]string{repo1})
	if err != nil {
		t.Fatal(err)
	}

	// Set up a second repo
	repo2 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".expedition": {produces: []string{"report"}, consumes: []string{"specification"}},
	})

	// when
	orphans, err := Add(result.Config, repo2)

	// then
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(result.Config.Repositories) != 2 {
		t.Errorf("repositories = %d, want 2", len(result.Config.Repositories))
	}
	_ = orphans
}

func TestAdd_DuplicateRepository(t *testing.T) {
	repo := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren": {produces: []string{"specification"}},
	})
	result, err := Init([]string{repo})
	if err != nil {
		t.Fatal(err)
	}

	// when — add the same repo again
	_, err = Add(result.Config, repo)
	if err == nil {
		t.Fatal("expected error for duplicate repository")
	}
}

func TestRemove_ExistingRepository(t *testing.T) {
	repo1 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren": {produces: []string{"specification"}},
	})
	repo2 := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".expedition": {produces: []string{"report"}},
	})

	result, err := Init([]string{repo1, repo2})
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

func TestSync_UpdatesEndpoints(t *testing.T) {
	repo := setupTestRepo(t, map[string]struct{ produces, consumes []string }{
		".siren": {produces: []string{"specification"}, consumes: []string{"feedback"}},
	})

	result, err := Init([]string{repo})
	if err != nil {
		t.Fatal(err)
	}

	// when — sync (even if nothing changed, should still work)
	orphans, err := Sync(result.Config)

	// then
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(result.Config.Repositories) != 1 {
		t.Errorf("repositories = %d, want 1", len(result.Config.Repositories))
	}
	_ = orphans
}
