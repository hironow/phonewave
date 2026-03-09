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

	dir := t.TempDir()
	configPath := filepath.Join(dir, domain.ConfigFile)

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
	dir := t.TempDir()
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "my-repo")
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	dir := t.TempDir()
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	dir := t.TempDir()
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	dir := t.TempDir()
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	dir := t.TempDir()
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(runDir, 0755)
	configPath := filepath.Join(dir, domain.ConfigFile)
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

	// then — resolved.yaml should exist in .run/ (relative to config dir) and contain routes
	resolvedPath := filepath.Join(dir, ".run", domain.ResolvedStateFile)
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
	dir := t.TempDir()
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(runDir, 0755)

	// Write manifest (no routes)
	manifest := `repositories:
  - path: .
    endpoints:
      - dir: .siren
        produces: [specification]
        consumes: [design-feedback]
`
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	dir := t.TempDir()
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
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	// given — old-style phonewave.yaml with routes inline
	dir := t.TempDir()
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
	configPath := filepath.Join(dir, domain.ConfigFile)
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
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "project")
	configPath := filepath.Join(dir, domain.ConfigFile)
	cfg := &domain.Config{
		Routes: []domain.RouteConfig{
			{Kind: "report", From: ".expedition/outbox", To: []string{".siren/inbox"}, RepoPath: repoPath},
		},
	}

	// when
	if err := session.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// then
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, dir) {
		t.Errorf("route repo_path should be relative, found absolute directory in:\n%s", content)
	}
}
