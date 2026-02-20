package phonewave

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EndpointDiff describes a change to an endpoint during sync.
type EndpointDiff struct {
	Repo   string
	Dir    string
	Change string // "added", "removed", "changed"
}

// RouteDiff describes a change to a route during sync.
type RouteDiff struct {
	Kind   string
	From   string
	Change string // "added", "removed"
}

// SyncReport holds the result of a sync operation including change diffs.
type SyncReport struct {
	Orphans         OrphanReport
	EndpointChanges []EndpointDiff
	RouteChanges    []RouteDiff
	RepoCount       int
	TotalRoutes     int
}

// snapshotEndpoints returns a map of "repoBase/dir" → EndpointConfig for diffing.
func snapshotEndpoints(cfg *Config) map[string]EndpointConfig {
	snap := make(map[string]EndpointConfig)
	for _, repo := range cfg.Repositories {
		base := filepath.Base(repo.Path)
		for _, ep := range repo.Endpoints {
			key := base + "/" + ep.Dir
			snap[key] = ep
		}
	}
	return snap
}

// snapshotRoutes returns a map of "kind:from" → RouteConfig for diffing.
func snapshotRoutes(cfg *Config) map[string]RouteConfig {
	snap := make(map[string]RouteConfig)
	for _, r := range cfg.Routes {
		key := r.Kind + ":" + r.From
		snap[key] = r
	}
	return snap
}

// diffEndpoints computes the difference between old and new endpoint snapshots.
func diffEndpoints(old, new_ map[string]EndpointConfig) []EndpointDiff {
	var diffs []EndpointDiff

	for key, newEp := range new_ {
		if oldEp, exists := old[key]; !exists {
			parts := strings.SplitN(key, "/", 2)
			repo, dir := parts[0], parts[1]
			diffs = append(diffs, EndpointDiff{Repo: repo, Dir: dir, Change: "added"})
		} else if !endpointEqual(oldEp, newEp) {
			parts := strings.SplitN(key, "/", 2)
			repo, dir := parts[0], parts[1]
			diffs = append(diffs, EndpointDiff{Repo: repo, Dir: dir, Change: "changed"})
		}
	}

	for key := range old {
		if _, exists := new_[key]; !exists {
			parts := strings.SplitN(key, "/", 2)
			repo, dir := parts[0], parts[1]
			diffs = append(diffs, EndpointDiff{Repo: repo, Dir: dir, Change: "removed"})
		}
	}

	return diffs
}

// endpointEqual checks if two EndpointConfigs have the same produces/consumes.
func endpointEqual(a, b EndpointConfig) bool {
	return slicesEqual(a.Produces, b.Produces) && slicesEqual(a.Consumes, b.Consumes)
}

// slicesEqual checks if two string slices have the same elements (order-insensitive).
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := make([]string, len(a))
	sb := make([]string, len(b))
	copy(sa, a)
	copy(sb, b)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// diffRoutes computes the difference between old and new route snapshots.
func diffRoutes(old, new_ map[string]RouteConfig) []RouteDiff {
	var diffs []RouteDiff

	for key, r := range new_ {
		if _, exists := old[key]; !exists {
			diffs = append(diffs, RouteDiff{Kind: r.Kind, From: r.From, Change: "added"})
		}
	}

	for key, r := range old {
		if _, exists := new_[key]; !exists {
			diffs = append(diffs, RouteDiff{Kind: r.Kind, From: r.From, Change: "removed"})
		}
	}

	return diffs
}

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

	orphans := DetectOrphansPerRepo(cfg)

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

	orphans := DetectOrphansPerRepo(cfg)
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

	orphans := DetectOrphansPerRepo(cfg)
	return &orphans, nil
}

// Sync re-scans all repositories in the config, computes diffs, and updates endpoints/routes.
func Sync(cfg *Config) (*SyncReport, error) {
	// Snapshot before re-scan
	oldEndpoints := snapshotEndpoints(cfg)
	oldRoutes := snapshotRoutes(cfg)

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

	// Snapshot after re-scan and diff
	newEndpoints := snapshotEndpoints(cfg)
	newRoutes := snapshotRoutes(cfg)

	orphans := DetectOrphansPerRepo(cfg)
	return &SyncReport{
		Orphans:         orphans,
		EndpointChanges: diffEndpoints(oldEndpoints, newEndpoints),
		RouteChanges:    diffRoutes(oldRoutes, newRoutes),
		RepoCount:       len(cfg.Repositories),
		TotalRoutes:     len(cfg.Routes),
	}, nil
}
