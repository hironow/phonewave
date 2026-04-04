package domain

import (
	"time"
)

// Endpoint represents a discovered tool endpoint within a repository.
type Endpoint struct {
	Dir      string   // dot-directory name, e.g. ".siren"
	Produces []string // list of kind values this endpoint produces
	Consumes []string // list of kind values this endpoint consumes
}

// Config is the top-level phonewave config.yaml structure.
type Config struct {
	LastSynced   time.Time     `yaml:"last_synced"`
	Repositories []RepoConfig  `yaml:"repositories"`
	Routes       []RouteConfig `yaml:"routes"`
}

// RepoConfig holds configuration for a single repository.
type RepoConfig struct {
	Path      string           `yaml:"path"`
	Endpoints []EndpointConfig `yaml:"endpoints"`
}

// EndpointConfig holds configuration for a single endpoint within a repo.
type EndpointConfig struct {
	Dir      string   `yaml:"dir"`
	Produces []string `yaml:"produces,flow"`
	Consumes []string `yaml:"consumes,flow"`
}

// ApproverConfig describes how approval behavior is configured.
// Implemented by FlagApproverConfig. Used by session.BuildApprover.
type ApproverConfig interface {
	IsAutoApprove() bool
	ApproveCmdString() string
}

// FlagApproverConfig adapts CLI flag values to the ApproverConfig interface.
type FlagApproverConfig struct {
	AutoApprove bool
	ApproveCmd  string
}

// IsAutoApprove reports whether auto-approve is enabled.
func (f FlagApproverConfig) IsAutoApprove() bool { return f.AutoApprove }

// ApproveCmdString returns the approval command string.
func (f FlagApproverConfig) ApproveCmdString() string { return f.ApproveCmd }

// AddResult holds the result of an add operation.
type AddResult struct {
	Orphans    OrphanReport
	Warnings   []string
	RouteCount int
}

// InitResult holds the result of an init operation.
type InitResult struct {
	Config    *Config
	Orphans   OrphanReport
	RepoCount int
	Warnings  []string
}

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
	Warnings        []string
}

// RouteConfig holds a derived routing rule.
type RouteConfig struct {
	Kind     string   `yaml:"kind"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to,flow"`
	Scope    string   `yaml:"scope"`
	RepoPath string   `yaml:"repo_path"`
}

// AddRepository appends a new repository with its endpoints to the config.
func (c *Config) AddRepository(path string, endpoints []Endpoint) {
	repo := RepoConfig{Path: path}
	for _, ep := range endpoints {
		repo.Endpoints = append(repo.Endpoints, EndpointConfig{
			Dir:      ep.Dir,
			Produces: ep.Produces,
			Consumes: ep.Consumes,
		})
	}
	c.Repositories = append(c.Repositories, repo)
}

// RemoveRepository removes a repository by path. Returns true if found.
func (c *Config) RemoveRepository(path string) bool {
	for i, repo := range c.Repositories {
		if repo.Path == path {
			c.Repositories = append(c.Repositories[:i], c.Repositories[i+1:]...)
			return true
		}
	}
	return false
}

// UpdateRoutes re-derives routes from all endpoints and updates the config.
func (c *Config) UpdateRoutes() {
	// Derive routes per repository
	var allRoutes []RouteConfig
	for _, repo := range c.Repositories {
		var endpoints []Endpoint
		for _, ep := range repo.Endpoints {
			endpoints = append(endpoints, Endpoint{
				Dir:      ep.Dir,
				Produces: ep.Produces,
				Consumes: ep.Consumes,
			})
		}
		routes := DeriveRoutes(endpoints)
		for _, r := range routes {
			allRoutes = append(allRoutes, RouteConfig{
				Kind:     r.Kind,
				From:     r.From,
				To:       r.To,
				Scope:    r.Scope,
				RepoPath: repo.Path,
			})
		}
	}
	c.Routes = allRoutes
}
