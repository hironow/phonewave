package session

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
)

// ResolveRoutes converts Config routes (relative paths) into ResolvedRoutes
// (absolute paths) that the delivery pipeline can use directly.
func ResolveRoutes(cfg *domain.Config) ([]domain.ResolvedRoute, error) {
	var resolved []domain.ResolvedRoute

	for _, route := range cfg.Routes {
		repoPath := route.RepoPath
		if repoPath == "" {
			// Fallback: derive repo from endpoint directory when RepoPath is unset
			repo, err := findRepoForRoute(cfg, route.From)
			if err != nil {
				return nil, err
			}
			repoPath = repo.Path
		}

		fromAbs := filepath.Join(repoPath, route.From)
		var toAbs []string
		for _, to := range route.To {
			toAbs = append(toAbs, filepath.Join(repoPath, to))
		}

		resolved = append(resolved, domain.ResolvedRoute{
			Kind:       route.Kind,
			FromOutbox: fromAbs,
			ToInboxes:  toAbs,
		})
	}

	return resolved, nil
}

// findRepoForRoute locates the repository that contains the given relative
// outbox path (e.g. ".siren/outbox").
func findRepoForRoute(cfg *domain.Config, fromPath string) (*domain.RepoConfig, error) {
	parts := strings.SplitN(fromPath, string(filepath.Separator), 2)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid from path: %q", fromPath)
	}
	dotDir := parts[0]

	for i := range cfg.Repositories {
		for _, ep := range cfg.Repositories[i].Endpoints {
			if ep.Dir == dotDir {
				return &cfg.Repositories[i], nil
			}
		}
	}
	return nil, fmt.Errorf("no repository found for route from %q", fromPath)
}

// CollectOutboxDirs returns all absolute outbox directory paths from endpoints
// that produce at least one kind. Consume-only endpoints are excluded because
// they may not have an outbox directory.
func CollectOutboxDirs(cfg *domain.Config) []string {
	var dirs []string
	for _, repo := range cfg.Repositories {
		for _, ep := range repo.Endpoints {
			if len(ep.Produces) > 0 {
				dirs = append(dirs, filepath.Join(repo.Path, ep.Dir, "outbox"))
			}
		}
	}
	return dirs
}
