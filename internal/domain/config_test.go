package domain_test

import (
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestConfigAddRepository(t *testing.T) {
	// given — existing config with one repo
	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{Path: "/repo-a", Endpoints: []domain.EndpointConfig{{Dir: ".siren"}}},
		},
	}

	// when — add a new repo
	newEndpoints := []domain.Endpoint{
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
	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
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
	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{{Path: "/repo-a"}},
	}
	removed := cfg.RemoveRepository("/repo-not-exist")
	if removed {
		t.Fatal("expected false for non-existent repo")
	}
}
