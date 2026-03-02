package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave"
)

func TestConfigRoundTrip(t *testing.T) {
	// given
	cfg := &phonewave.Config{
		LastSynced: time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Repositories: []phonewave.RepoConfig{
			{
				Path: "/home/user/repo-a",
				Endpoints: []phonewave.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification", "feedback"}},
				},
			},
		},
		Routes: []phonewave.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, phonewave.ConfigFile)

	// when — write
	if err := WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// then — read back
	loaded, err := LoadConfig(configPath)
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
	_, err := LoadConfig("/nonexistent/phonewave.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestWriteConfig_CreatesYAMLFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, phonewave.ConfigFile)
	cfg := &phonewave.Config{
		LastSynced: time.Now().UTC(),
		Repositories: []phonewave.RepoConfig{
			{
				Path: "/test",
				Endpoints: []phonewave.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
				},
			},
		},
	}

	if err := WriteConfig(configPath, cfg); err != nil {
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
	configPath := filepath.Join(dir, phonewave.ConfigFile)
	cfg := &phonewave.Config{
		Repositories: []phonewave.RepoConfig{
			{Path: repoPath, Endpoints: []phonewave.EndpointConfig{{Dir: ".siren"}}},
		},
		Routes: []phonewave.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".siren/inbox"}, RepoPath: repoPath},
		},
	}

	// when
	if err := WriteConfig(configPath, cfg); err != nil {
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
	configPath := filepath.Join(dir, phonewave.ConfigFile)
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
	cfg, err := LoadConfig(configPath)
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
	configPath := filepath.Join(dir, phonewave.ConfigFile)
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
	cfg, err := LoadConfig(configPath)
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

func TestWriteConfig_RoutesAlsoRelative(t *testing.T) {
	// given — config with routes containing absolute repo_path
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "project")
	configPath := filepath.Join(dir, phonewave.ConfigFile)
	cfg := &phonewave.Config{
		Routes: []phonewave.RouteConfig{
			{Kind: "report", From: ".expedition/outbox", To: []string{".siren/inbox"}, RepoPath: repoPath},
		},
	}

	// when
	if err := WriteConfig(configPath, cfg); err != nil {
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
