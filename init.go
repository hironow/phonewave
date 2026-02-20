package phonewave

import (
	"fmt"
	"path/filepath"
	"time"
)

// InitResult holds the result of an init operation.
type InitResult struct {
	Config    *Config
	Orphans   OrphanReport
	RepoCount int
}

// Init scans multiple repositories, derives routes, and generates a Config.
func Init(repoPaths []string) (*InitResult, error) {
	cfg := &Config{
		LastSynced: time.Now().UTC(),
	}

	for _, repoPath := range repoPaths {
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %w", repoPath, err)
		}

		endpoints, err := ScanRepository(absPath)
		if err != nil {
			return nil, fmt.Errorf("scan %q: %w", absPath, err)
		}

		cfg.AddRepository(absPath, endpoints)
	}

	cfg.UpdateRoutes()

	orphans := DetectOrphans(cfg.AllEndpoints())

	return &InitResult{
		Config:    cfg,
		Orphans:   orphans,
		RepoCount: len(repoPaths),
	}, nil
}

// Add scans a new repository and adds it to an existing config.
func Add(cfg *Config, repoPath string) (*OrphanReport, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", repoPath, err)
	}

	// Check for duplicate
	for _, repo := range cfg.Repositories {
		if repo.Path == absPath {
			return nil, fmt.Errorf("repository %q already exists in config", absPath)
		}
	}

	endpoints, err := ScanRepository(absPath)
	if err != nil {
		return nil, fmt.Errorf("scan %q: %w", absPath, err)
	}

	cfg.AddRepository(absPath, endpoints)
	cfg.UpdateRoutes()
	cfg.LastSynced = time.Now().UTC()

	orphans := DetectOrphans(cfg.AllEndpoints())
	return &orphans, nil
}

// Remove removes a repository from the config and re-derives routes.
func Remove(cfg *Config, repoPath string) (*OrphanReport, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", repoPath, err)
	}

	if !cfg.RemoveRepository(absPath) {
		return nil, fmt.Errorf("repository %q not found in config", absPath)
	}

	cfg.UpdateRoutes()
	cfg.LastSynced = time.Now().UTC()

	orphans := DetectOrphans(cfg.AllEndpoints())
	return &orphans, nil
}

// Sync re-scans all repositories in the config and updates endpoints/routes.
func Sync(cfg *Config) (*OrphanReport, error) {
	var newRepos []RepoConfig

	for _, repo := range cfg.Repositories {
		endpoints, err := ScanRepository(repo.Path)
		if err != nil {
			return nil, fmt.Errorf("scan %q: %w", repo.Path, err)
		}

		rc := RepoConfig{Path: repo.Path}
		for _, ep := range endpoints {
			rc.Endpoints = append(rc.Endpoints, EndpointConfig{
				Dir:      ep.Dir,
				Produces: ep.Produces,
				Consumes: ep.Consumes,
			})
		}
		newRepos = append(newRepos, rc)
	}

	cfg.Repositories = newRepos
	cfg.UpdateRoutes()
	cfg.LastSynced = time.Now().UTC()

	orphans := DetectOrphans(cfg.AllEndpoints())
	return &orphans, nil
}
