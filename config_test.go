package phonewave

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigRoundTrip(t *testing.T) {
	// given
	cfg := &Config{
		LastSynced: time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Repositories: []RepoConfig{
			{
				Path: "/home/user/repo-a",
				Endpoints: []EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification", "feedback"}},
				},
			},
		},
		Routes: []RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, ConfigFile)

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

func TestConfigAddRepository(t *testing.T) {
	// given — existing config with one repo
	cfg := &Config{
		Repositories: []RepoConfig{
			{Path: "/repo-a", Endpoints: []EndpointConfig{{Dir: ".siren"}}},
		},
	}

	// when — add a new repo
	newEndpoints := []Endpoint{
		{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
	}
	cfg.AddRepository("/repo-b", newEndpoints)

	// then
	if len(cfg.Repositories) != 2 {
		t.Fatalf("want 2 repositories, got %d", len(cfg.Repositories))
	}
	if cfg.Repositories[1].Path != "/repo-b" {
		t.Errorf("path = %q, want /repo-b", cfg.Repositories[1].Path)
	}
}

func TestConfigRemoveRepository(t *testing.T) {
	// given
	cfg := &Config{
		Repositories: []RepoConfig{
			{Path: "/repo-a"},
			{Path: "/repo-b"},
		},
	}

	// when
	removed := cfg.RemoveRepository("/repo-a")

	// then
	if !removed {
		t.Fatal("expected RemoveRepository to return true")
	}
	if len(cfg.Repositories) != 1 {
		t.Fatalf("want 1 repository, got %d", len(cfg.Repositories))
	}
	if cfg.Repositories[0].Path != "/repo-b" {
		t.Errorf("remaining repo = %q, want /repo-b", cfg.Repositories[0].Path)
	}
}

func TestConfigRemoveRepository_NotFound(t *testing.T) {
	cfg := &Config{
		Repositories: []RepoConfig{{Path: "/repo-a"}},
	}
	removed := cfg.RemoveRepository("/repo-not-exist")
	if removed {
		t.Fatal("expected false for non-existent repo")
	}
}

func TestWriteConfig_CreatesYAMLFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ConfigFile)
	cfg := &Config{
		LastSynced: time.Now().UTC(),
		Repositories: []RepoConfig{
			{
				Path: "/test",
				Endpoints: []EndpointConfig{
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
