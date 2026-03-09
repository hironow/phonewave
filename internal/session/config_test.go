package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

// stateConfigPath creates a .phonewave/config.yaml inside a temp dir,
// mirroring the real directory structure. Returns (projectRoot, configPath).
func stateConfigPath(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	os.MkdirAll(stateDir, 0755)
	return dir, filepath.Join(stateDir, domain.ConfigFile)
}

func TestConfigRoundTrip(t *testing.T) {
	// given
	cfg := &domain.Config{
		LastSynced: time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Repositories: []domain.RepoConfig{
			{
				Path: "/home/user/repo-a",
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification", "design-feedback"}},
				},
			},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	_, configPath := stateConfigPath(t)

	// when — write
	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// then — read back
	loaded, err := session.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(loaded.Repositories) != 1 {
		t.Fatalf("want 1 repository, got %d", len(loaded.Repositories))
	}
	if loaded.Repositories[0].Path != "/home/user/repo-a" {
		t.Errorf("repo path = %q, want %q", loaded.Repositories[0].Path, "/home/user/repo-a")
	}
	if len(loaded.Repositories[0].Endpoints) != 2 {
		t.Errorf("want 2 endpoints, got %d", len(loaded.Repositories[0].Endpoints))
	}
	if len(loaded.Routes) != 1 {
		t.Fatalf("want 1 route, got %d", len(loaded.Routes))
	}
	if loaded.Routes[0].Kind != "specification" {
		t.Errorf("route kind = %q, want %q", loaded.Routes[0].Kind, "specification")
	}
	if loaded.LastSynced.IsZero() {
		t.Error("last_synced should not be zero")
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := session.LoadConfig("/nonexistent/phonewave.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestWriteConfig_CreatesYAMLFile(t *testing.T) {
	_, configPath := stateConfigPath(t)
	cfg := &domain.Config{
		LastSynced: time.Now().UTC(),
		Repositories: []domain.RepoConfig{
			{
				Path: "/test",
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
				},
			},
		},
	}

	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	content := string(data)
	// Should contain comment header
	if len(content) == 0 {
		t.Fatal("config file is empty")
	}
}

func TestWriteConfig_StoresRelativePaths(t *testing.T) {
	// given — config with absolute repo paths
	dir, configPath := stateConfigPath(t)
	repoPath := filepath.Join(dir, "my-repo")
	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{Path: repoPath, Endpoints: []domain.EndpointConfig{{Dir: ".siren"}}},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".siren/inbox"}, RepoPath: repoPath},
		},
	}

	// when
	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// then — on-disk YAML should contain relative path, not absolute
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	if strings.Contains(content, dir) {
		t.Errorf("YAML should not contain absolute directory %q, got:\n%s", dir, content)
	}
	if !strings.Contains(content, "my-repo") {
		t.Errorf("YAML should contain relative path 'my-repo', got:\n%s", content)
	}

	// original cfg should not be mutated
	if cfg.Repositories[0].Path != repoPath {
		t.Errorf("WriteConfig mutated original config: path = %q, want %q", cfg.Repositories[0].Path, repoPath)
	}
}

func TestLoadConfig_ResolvesRelativePaths(t *testing.T) {
	// given — YAML with relative paths
	dir, configPath := stateConfigPath(t)
	yamlContent := `repositories:
  - path: my-repo
    endpoints:
      - dir: .siren
routes:
  - kind: specification
    from: .siren/outbox
    to: [.siren/inbox]
    repo_path: my-repo
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// when
	cfg, err := session.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// then — paths should be resolved to absolute
	expectedPath := filepath.Join(dir, "my-repo")
	if cfg.Repositories[0].Path != expectedPath {
		t.Errorf("repo path = %q, want %q", cfg.Repositories[0].Path, expectedPath)
	}
	if cfg.Routes[0].RepoPath != expectedPath {
		t.Errorf("route repo_path = %q, want %q", cfg.Routes[0].RepoPath, expectedPath)
	}
}

func TestLoadConfig_BackwardCompat_AbsolutePaths(t *testing.T) {
	// given — YAML with absolute paths (legacy format)
	_, configPath := stateConfigPath(t)
	yamlContent := `repositories:
  - path: /absolute/path/to/repo
    endpoints:
      - dir: .siren
routes:
  - kind: specification
    from: .siren/outbox
    to: [.siren/inbox]
    repo_path: /absolute/path/to/repo
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// when
	cfg, err := session.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// then — absolute paths should be kept as-is
	if cfg.Repositories[0].Path != "/absolute/path/to/repo" {
		t.Errorf("repo path = %q, want %q", cfg.Repositories[0].Path, "/absolute/path/to/repo")
	}
	if cfg.Routes[0].RepoPath != "/absolute/path/to/repo" {
		t.Errorf("route repo_path = %q, want %q", cfg.Routes[0].RepoPath, "/absolute/path/to/repo")
	}
}

func TestWriteConfig_ManifestExcludesRoutes(t *testing.T) {
	// given — config with routes
	dir, configPath := stateConfigPath(t)
	cfg := &domain.Config{
		LastSynced: time.Now().UTC(),
		Repositories: []domain.RepoConfig{
			{Path: dir, Endpoints: []domain.EndpointConfig{{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}}}},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository", RepoPath: dir},
		},
	}

	// when
	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// then — config.yaml should NOT contain routes or last_synced
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "routes:") {
		t.Errorf("config.yaml should not contain routes (moved to resolved.yaml), got:\n%s", content)
	}
	if strings.Contains(content, "last_synced:") {
		t.Errorf("config.yaml should not contain last_synced (moved to resolved.yaml), got:\n%s", content)
	}
}

func TestWriteConfig_ResolvedStateContainsRoutes(t *testing.T) {
	// given — config with routes
	dir, configPath := stateConfigPath(t)
	stateDir := filepath.Dir(configPath)
	cfg := &domain.Config{
		LastSynced: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
		Repositories: []domain.RepoConfig{
			{Path: dir, Endpoints: []domain.EndpointConfig{{Dir: ".siren", Produces: []string{"specification"}}}},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository", RepoPath: dir},
		},
	}

	// when
	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// then — resolved.yaml should exist in .run/ (relative to state dir) and contain routes
	resolvedPath := filepath.Join(stateDir, ".run", domain.ResolvedStateFile)
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatalf("read resolved.yaml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "routes:") {
		t.Errorf("resolved.yaml should contain routes, got:\n%s", content)
	}
	if !strings.Contains(content, "last_synced:") {
		t.Errorf("resolved.yaml should contain last_synced, got:\n%s", content)
	}
}

func TestLoadConfig_MergesResolvedState(t *testing.T) {
	// given — separate manifest and resolved state
	_, configPath := stateConfigPath(t)
	stateDir := filepath.Dir(configPath)
	runDir := filepath.Join(stateDir, ".run")
	os.MkdirAll(runDir, 0755)

	// Write manifest (no routes)
	manifest := `repositories:
  - path: .
    endpoints:
      - dir: .siren
        produces: [specification]
        consumes: [design-feedback]
`
	os.WriteFile(configPath, []byte(manifest), 0644)

	// Write resolved state
	resolved := `last_synced: 2026-03-08T12:00:00Z
routes:
  - kind: specification
    from: .siren/outbox
    to: [.expedition/inbox]
    scope: same_repository
    repo_path: .
`
	os.WriteFile(filepath.Join(runDir, domain.ResolvedStateFile), []byte(resolved), 0644)

	// when
	cfg, err := session.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// then — merged config should have both endpoints and routes
	if len(cfg.Repositories) != 1 {
		t.Fatalf("want 1 repo, got %d", len(cfg.Repositories))
	}
	if len(cfg.Routes) != 1 {
		t.Fatalf("want 1 route, got %d", len(cfg.Routes))
	}
	if cfg.Routes[0].Kind != "specification" {
		t.Errorf("route kind = %q, want specification", cfg.Routes[0].Kind)
	}
	if cfg.LastSynced.IsZero() {
		t.Error("last_synced should not be zero")
	}
}

func TestLoadConfig_GracefulWithoutResolvedState(t *testing.T) {
	// given — manifest only, no resolved.yaml
	_, configPath := stateConfigPath(t)
	manifest := `repositories:
  - path: .
    endpoints:
      - dir: .siren
        produces: [specification]
        consumes: [design-feedback]
      - dir: .expedition
        consumes: [specification]
        produces: [report]
`
	os.WriteFile(configPath, []byte(manifest), 0644)

	// when
	cfg, err := session.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// then — routes should be derived from endpoints
	if len(cfg.Routes) == 0 {
		t.Error("LoadConfig should derive routes when resolved.yaml is missing")
	}
}

func TestLoadConfig_BackwardCompat_OldFormatWithRoutes(t *testing.T) {
	// given — old-style config with routes inline
	_, configPath := stateConfigPath(t)
	oldYaml := `last_synced: 2026-03-07T03:40:13Z
repositories:
  - path: .
    endpoints:
      - dir: .siren
        produces: [specification]
        consumes: [design-feedback]
routes:
  - kind: specification
    from: .siren/outbox
    to: [.expedition/inbox]
    scope: same_repository
    repo_path: .
`
	os.WriteFile(configPath, []byte(oldYaml), 0644)

	// when
	cfg, err := session.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// then — should still work (backward compatible)
	if len(cfg.Routes) != 1 {
		t.Fatalf("want 1 route from old format, got %d", len(cfg.Routes))
	}
	if cfg.Routes[0].Kind != "specification" {
		t.Errorf("route kind = %q, want specification", cfg.Routes[0].Kind)
	}
}

func TestWriteConfig_RoutesAlsoRelative(t *testing.T) {
	// given — config with routes containing absolute repo_path
	projectRoot, configPath := stateConfigPath(t)
	repoPath := filepath.Join(projectRoot, "project")
	cfg := &domain.Config{
		Routes: []domain.RouteConfig{
			{Kind: "report", From: ".expedition/outbox", To: []string{".siren/inbox"}, RepoPath: repoPath},
		},
	}

	// when
	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// then — check resolved state file (routes are written there, not config.yaml)
	resolvedPath := filepath.Join(filepath.Dir(configPath), ".run", domain.ResolvedStateFile)
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatalf("read resolved state: %v", err)
	}
	content := string(data)
	if strings.Contains(content, projectRoot) {
		t.Errorf("route repo_path should be relative, found absolute directory in:\n%s", content)
	}
}

func TestMigrateConfigIfNeeded_MovesLegacyFile(t *testing.T) {
	// given — legacy phonewave.yaml at project root, no .phonewave/config.yaml
	projectRoot := t.TempDir()
	legacyContent := "# phonewave.yaml\nrepositories:\n  - path: .\n"
	legacyPath := filepath.Join(projectRoot, domain.LegacyConfigFile)
	os.WriteFile(legacyPath, []byte(legacyContent), 0644)

	// when
	err := session.MigrateConfigIfNeeded(projectRoot)

	// then
	if err != nil {
		t.Fatalf("MigrateConfigIfNeeded: %v", err)
	}
	// legacy file should be gone
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("legacy phonewave.yaml should be removed after migration")
	}
	// new config should exist
	newPath := domain.DefaultConfigPath(projectRoot)
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("migrated config should exist: %v", err)
	}
	if string(data) != legacyContent {
		t.Errorf("migrated content mismatch: got %q", string(data))
	}
	// .gitignore should include !config.yaml
	gitignore, err := os.ReadFile(filepath.Join(projectRoot, domain.StateDir, ".gitignore"))
	if err != nil {
		t.Fatalf("gitignore should exist: %v", err)
	}
	if !strings.Contains(string(gitignore), "!config.yaml") {
		t.Error("gitignore should include !config.yaml after migration")
	}
}

func TestMigrateConfigIfNeeded_NoLegacyFile(t *testing.T) {
	// given — no legacy file exists
	projectRoot := t.TempDir()

	// when
	err := session.MigrateConfigIfNeeded(projectRoot)

	// then — should be a no-op
	if err != nil {
		t.Fatalf("MigrateConfigIfNeeded should be no-op: %v", err)
	}
}

func TestMigrateConfigIfNeeded_BothExist_RemovesLegacy(t *testing.T) {
	// given — both legacy and new config exist
	projectRoot := t.TempDir()
	stateDir := filepath.Join(projectRoot, domain.StateDir)
	os.MkdirAll(stateDir, 0755)

	legacyPath := filepath.Join(projectRoot, domain.LegacyConfigFile)
	os.WriteFile(legacyPath, []byte("old"), 0644)
	newPath := domain.DefaultConfigPath(projectRoot)
	os.WriteFile(newPath, []byte("new"), 0644)

	// when
	err := session.MigrateConfigIfNeeded(projectRoot)

	// then — legacy removed, new preserved
	if err != nil {
		t.Fatalf("MigrateConfigIfNeeded: %v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("legacy file should be removed when both exist")
	}
	data, _ := os.ReadFile(newPath)
	if string(data) != "new" {
		t.Error("new config should be preserved")
	}
}
