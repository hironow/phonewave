package phonewave

import (
	"path/filepath"
	"sort"
)

// Route represents a derived routing rule for a D-Mail kind.
type Route struct {
	Kind  string   // D-Mail kind (e.g. "specification", "report", "feedback")
	From  string   // source outbox path relative to repository (e.g. ".siren/outbox")
	To    []string // target inbox paths relative to repository (e.g. [".expedition/inbox"])
	Scope string   // "same_repository" or "cross_repository"
}

// OrphanReport contains kinds that are produced but not consumed, or vice versa.
type OrphanReport struct {
	UnconsumedKinds []string // produced but not consumed by any endpoint
	UnproducedKinds []string // consumed but not produced by any endpoint
}

// DeriveRoutes matches produces to consumes across endpoints within the
// same repository, generating one Route per (producer, kind) pair.
// A producer's kind must have at least one consumer to generate a route.
func DeriveRoutes(endpoints []Endpoint) []Route {
	// Build consumer index: kind → list of endpoint dirs that consume it
	consumers := make(map[string][]string)
	for _, ep := range endpoints {
		for _, kind := range ep.Consumes {
			consumers[kind] = append(consumers[kind], ep.Dir)
		}
	}

	var routes []Route
	for _, ep := range endpoints {
		for _, kind := range ep.Produces {
			targets, ok := consumers[kind]
			if !ok || len(targets) == 0 {
				continue // orphaned producer — no route
			}

			// Build target inbox paths, excluding self-delivery
			var to []string
			for _, targetDir := range targets {
				if targetDir == ep.Dir {
					continue // don't deliver to yourself
				}
				to = append(to, filepath.Join(targetDir, "inbox"))
			}

			if len(to) == 0 {
				continue
			}

			routes = append(routes, Route{
				Kind:  kind,
				From:  filepath.Join(ep.Dir, "outbox"),
				To:    to,
				Scope: "same_repository",
			})
		}
	}

	return routes
}

// DetectOrphansPerRepo runs orphan detection per repository, matching the
// same_repository routing scope. A kind produced in repo A and consumed
// only in repo B will be reported as orphaned in both.
func DetectOrphansPerRepo(cfg *Config) OrphanReport {
	unconsumedSet := make(map[string]bool)
	unproducedSet := make(map[string]bool)

	for _, repo := range cfg.Repositories {
		var endpoints []Endpoint
		for _, ep := range repo.Endpoints {
			endpoints = append(endpoints, Endpoint{
				Dir:      ep.Dir,
				Produces: ep.Produces,
				Consumes: ep.Consumes,
			})
		}
		report := DetectOrphans(endpoints)
		for _, kind := range report.UnconsumedKinds {
			unconsumedSet[kind] = true
		}
		for _, kind := range report.UnproducedKinds {
			unproducedSet[kind] = true
		}
	}

	var result OrphanReport
	for kind := range unconsumedSet {
		result.UnconsumedKinds = append(result.UnconsumedKinds, kind)
	}
	for kind := range unproducedSet {
		result.UnproducedKinds = append(result.UnproducedKinds, kind)
	}
	sort.Strings(result.UnconsumedKinds)
	sort.Strings(result.UnproducedKinds)
	return result
}

// DetectOrphans finds kinds that are produced but not consumed, or consumed
// but not produced, within the given endpoints.
func DetectOrphans(endpoints []Endpoint) OrphanReport {
	produced := make(map[string]bool)
	consumed := make(map[string]bool)

	for _, ep := range endpoints {
		for _, kind := range ep.Produces {
			produced[kind] = true
		}
		for _, kind := range ep.Consumes {
			consumed[kind] = true
		}
	}

	var report OrphanReport

	for kind := range produced {
		if !consumed[kind] {
			report.UnconsumedKinds = append(report.UnconsumedKinds, kind)
		}
	}
	for kind := range consumed {
		if !produced[kind] {
			report.UnproducedKinds = append(report.UnproducedKinds, kind)
		}
	}

	sort.Strings(report.UnconsumedKinds)
	sort.Strings(report.UnproducedKinds)

	return report
}
