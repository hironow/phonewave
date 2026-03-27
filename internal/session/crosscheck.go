package session

import (
	"fmt"
	"strings"

	"github.com/hironow/phonewave/internal/domain"
)

// CheckRoutingConsistency verifies that D-Mail routing has no orphaned
// producers or consumers across the configured endpoints.
func CheckRoutingConsistency(cfg *domain.Config) []domain.UnifiedCheck {
	if cfg == nil {
		return []domain.UnifiedCheck{{
			Name:    "routing",
			Status:  "SKIP",
			Message: "no config loaded",
		}}
	}

	orphans := domain.DetectOrphansPerRepo(cfg)

	var checks []domain.UnifiedCheck

	if len(orphans.UnconsumedKinds) > 0 {
		checks = append(checks, domain.UnifiedCheck{
			Name:    "routing-unconsumed",
			Status:  "WARN",
			Message: fmt.Sprintf("produced but not consumed: %s", strings.Join(orphans.UnconsumedKinds, ", ")),
			Hint:    "Add a consumer endpoint for these D-Mail kinds",
		})
	}

	if len(orphans.UnproducedKinds) > 0 {
		checks = append(checks, domain.UnifiedCheck{
			Name:    "routing-unproduced",
			Status:  "WARN",
			Message: fmt.Sprintf("consumed but not produced: %s", strings.Join(orphans.UnproducedKinds, ", ")),
			Hint:    "Add a producer endpoint for these D-Mail kinds or remove the consumer",
		})
	}

	if len(checks) == 0 {
		// Count routes for the OK message
		var totalRoutes int
		for _, repo := range cfg.Repositories {
			var endpoints []domain.Endpoint
			for _, ep := range repo.Endpoints {
				endpoints = append(endpoints, domain.Endpoint{
					Dir: ep.Dir, Produces: ep.Produces, Consumes: ep.Consumes,
				})
			}
			totalRoutes += len(domain.DeriveRoutes(endpoints))
		}
		checks = append(checks, domain.UnifiedCheck{
			Name:    "routing",
			Status:  "OK",
			Message: fmt.Sprintf("%d route(s) resolved, no orphans", totalRoutes),
		})
	}

	return checks
}
