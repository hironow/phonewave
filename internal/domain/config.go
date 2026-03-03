package domain

import (
	"time"
)

// Config is the top-level phonewave.yaml structure.
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

// AllEndpoints returns all endpoints across all repositories as Endpoint values.
func (c *Config) AllEndpoints() []Endpoint {
	var all []Endpoint
	for _, repo := range c.Repositories {
		for _, ep := range repo.Endpoints {
			all = append(all, Endpoint{
				Dir:      ep.Dir,
				Produces: ep.Produces,
				Consumes: ep.Consumes,
			})
		}
	}
	return all
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
