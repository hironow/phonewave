package session

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	pond "github.com/alitto/pond/v2"
	"github.com/hironow/phonewave/internal/domain"
)

// snapshotEndpoints returns a map of "repoPath/dir" → EndpointConfig for diffing.
// Uses full repo path to avoid collisions between repos with the same basename.
func snapshotEndpoints(cfg *domain.Config) map[string]domain.EndpointConfig {
	snap := make(map[string]domain.EndpointConfig)
	for _, repo := range cfg.Repositories {
		for _, ep := range repo.Endpoints {
			key := repo.Path + "/" + ep.Dir // nosemgrep: adr0005-string-concat-file-path — map key, not file path [permanent]
			snap[key] = ep
		}
	}
	return snap
}

// snapshotRoutes returns a map of "repoPath:kind:from" → RouteConfig for diffing.
// Includes RepoPath to avoid collisions in multi-repo configs with overlapping kinds/paths.
func snapshotRoutes(cfg *domain.Config) map[string]domain.RouteConfig {
	snap := make(map[string]domain.RouteConfig)
	for _, r := range cfg.Routes {
		key := r.RepoPath + ":" + r.Kind + ":" + r.From
		snap[key] = r
	}
	return snap
}

// diffEndpoints computes the difference between old and new endpoint snapshots.
func diffEndpoints(old, new_ map[string]domain.EndpointConfig) []domain.EndpointDiff {
	var diffs []domain.EndpointDiff

	for key, newEp := range new_ {
		repo, dir := splitEndpointKey(key)
		if oldEp, exists := old[key]; !exists {
			diffs = append(diffs, domain.EndpointDiff{Repo: repo, Dir: dir, Change: "added"})
		} else if !endpointEqual(oldEp, newEp) {
			diffs = append(diffs, domain.EndpointDiff{Repo: repo, Dir: dir, Change: "changed"})
		}
	}

	for key := range old {
		if _, exists := new_[key]; !exists {
			repo, dir := splitEndpointKey(key)
			diffs = append(diffs, domain.EndpointDiff{Repo: repo, Dir: dir, Change: "removed"})
		}
	}

	return diffs
}

// splitEndpointKey splits a "repoPath/dir" key into repo basename and dir.
func splitEndpointKey(key string) (repo, dir string) {
	lastSlash := strings.LastIndex(key, "/")
	if lastSlash < 0 {
		return key, ""
	}
	return filepath.Base(key[:lastSlash]), key[lastSlash+1:]
}

// endpointEqual checks if two EndpointConfigs have the same produces/consumes.
func endpointEqual(a, b domain.EndpointConfig) bool {
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
func diffRoutes(old, new_ map[string]domain.RouteConfig) []domain.RouteDiff {
	var diffs []domain.RouteDiff

	for key, r := range new_ {
		if _, exists := old[key]; !exists {
			diffs = append(diffs, domain.RouteDiff{Kind: r.Kind, From: r.From, Change: "added"})
		}
	}

	for key, r := range old {
		if _, exists := new_[key]; !exists {
			diffs = append(diffs, domain.RouteDiff{Kind: r.Kind, From: r.From, Change: "removed"})
		}
	}

	return diffs
}

// repoScanResult holds the outcome of scanning a single repository.
type repoScanResult struct {
	absPath   string
	endpoints []domain.Endpoint
	err       error
}

// Init scans multiple repositories concurrently, derives routes, and generates
// a Config. Repository scanning is parallelized via a worker pool.
func Init(ctx context.Context, repoPaths []string) (*domain.InitResult, error) {
	cfg := &domain.Config{
		LastSynced: time.Now().UTC(),
	}

	pool := pond.NewResultPool[repoScanResult](runtime.NumCPU())
	group := pool.NewGroup()

	for _, repoPath := range repoPaths {
		repoPath := repoPath // capture for goroutine
		group.Submit(func() repoScanResult {
			absPath, err := filepath.Abs(repoPath)
			if err != nil {
				return repoScanResult{err: fmt.Errorf("invalid path %q: %w", repoPath, err)}
			}
			endpoints, err := ScanRepository(absPath)
			if err != nil {
				return repoScanResult{err: fmt.Errorf("scan %q: %w", absPath, err)}
			}
			return repoScanResult{absPath: absPath, endpoints: endpoints}
		})
	}

	// ResultTaskGroup.Wait() preserves submission order.
	scanResults, _ := group.Wait()
	pool.StopAndWait()

	for _, r := range scanResults {
		if r.err != nil {
			return nil, r.err
		}
		cfg.AddRepository(r.absPath, r.endpoints)
	}

	cfg.UpdateRoutes()

	orphans := domain.DetectOrphansPerRepo(cfg)

	return &domain.InitResult{
		Config:    cfg,
		Orphans:   orphans,
		RepoCount: len(repoPaths),
		Warnings:  collectSkillWarnings(ctx, cfg, ""),
	}, nil
}

// Add scans a new repository and adds it to an existing config.
func Add(ctx context.Context, cfg *domain.Config, repoPath string) (*domain.AddResult, error) {
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

	orphans := domain.DetectOrphansPerRepo(cfg)

	return &domain.AddResult{
		Orphans:  orphans,
		Warnings: collectSkillWarnings(ctx, cfg, absPath),
	}, nil
}

// Remove removes a repository from the config and re-derives routes.
func Remove(cfg *domain.Config, repoPath string) (*domain.OrphanReport, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", repoPath, err)
	}

	if !cfg.RemoveRepository(absPath) {
		return nil, fmt.Errorf("repository %q not found in config", absPath)
	}

	cfg.UpdateRoutes()
	cfg.LastSynced = time.Now().UTC()

	orphans := domain.DetectOrphansPerRepo(cfg)
	return &orphans, nil
}

// syncRepoResult holds the outcome of re-scanning a single repository.
type syncRepoResult struct {
	repoConfig domain.RepoConfig
	err        error
}

// Sync re-scans all repositories concurrently, computes diffs, and updates
// endpoints/routes. Repository scanning is parallelized via a worker pool.
func Sync(ctx context.Context, cfg *domain.Config) (*domain.SyncReport, error) {
	// Snapshot before re-scan
	oldEndpoints := snapshotEndpoints(cfg)
	oldRoutes := snapshotRoutes(cfg)

	pool := pond.NewResultPool[syncRepoResult](runtime.NumCPU())
	group := pool.NewGroup()

	for _, repo := range cfg.Repositories {
		repoPath := repo.Path // capture for goroutine
		group.Submit(func() syncRepoResult {
			endpoints, err := ScanRepository(repoPath)
			if err != nil {
				return syncRepoResult{err: fmt.Errorf("scan %q: %w", repoPath, err)}
			}

			rc := domain.RepoConfig{Path: repoPath}
			for _, ep := range endpoints {
				rc.Endpoints = append(rc.Endpoints, domain.EndpointConfig{
					Dir:      ep.Dir,
					Produces: ep.Produces,
					Consumes: ep.Consumes,
				})
			}
			return syncRepoResult{repoConfig: rc}
		})
	}

	// ResultTaskGroup.Wait() preserves submission order.
	scanResults, _ := group.Wait()
	pool.StopAndWait()

	var newRepos []domain.RepoConfig
	for _, r := range scanResults {
		if r.err != nil {
			return nil, r.err
		}
		newRepos = append(newRepos, r.repoConfig)
	}

	cfg.Repositories = newRepos
	cfg.UpdateRoutes()
	cfg.LastSynced = time.Now().UTC()

	// Snapshot after re-scan and diff
	newEndpoints := snapshotEndpoints(cfg)
	newRoutes := snapshotRoutes(cfg)

	orphans := domain.DetectOrphansPerRepo(cfg)

	return &domain.SyncReport{
		Orphans:         orphans,
		EndpointChanges: diffEndpoints(oldEndpoints, newEndpoints),
		RouteChanges:    diffRoutes(oldRoutes, newRoutes),
		RepoCount:       len(cfg.Repositories),
		TotalRoutes:     len(cfg.Routes),
		Warnings:        collectSkillWarnings(ctx, cfg, ""),
	}, nil
}
