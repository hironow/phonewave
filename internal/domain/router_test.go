package domain_test

import (
	"sort"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestDeriveRoutes_ThreeToolEcosystem(t *testing.T) {
	// given — the canonical Sightjack/Paintress/Amadeus setup
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
		{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification", "design-feedback"}},
		{Dir: ".gate", Produces: []string{"design-feedback"}, Consumes: []string{"report"}},
	}

	// when
	routes := domain.DeriveRoutes(endpoints)

	// then — 3 routes: specification, report, feedback
	if len(routes) != 3 {
		t.Fatalf("want 3 routes, got %d: %+v", len(routes), routes)
	}

	routeMap := make(map[string]domain.Route)
	for _, r := range routes {
		routeMap[r.Kind] = r
	}

	// specification: .siren/outbox → [.expedition/inbox]
	spec, ok := routeMap["specification"]
	if !ok {
		t.Fatal("missing route for kind=specification")
	}
	if spec.From != ".siren/outbox" {
		t.Errorf("specification.from = %q, want %q", spec.From, ".siren/outbox")
	}
	if len(spec.To) != 1 || spec.To[0] != ".expedition/inbox" {
		t.Errorf("specification.to = %v, want [.expedition/inbox]", spec.To)
	}

	// report: .expedition/outbox → [.gate/inbox]
	rep, ok := routeMap["report"]
	if !ok {
		t.Fatal("missing route for kind=report")
	}
	if rep.From != ".expedition/outbox" {
		t.Errorf("report.from = %q, want %q", rep.From, ".expedition/outbox")
	}
	if len(rep.To) != 1 || rep.To[0] != ".gate/inbox" {
		t.Errorf("report.to = %v, want [.gate/inbox]", rep.To)
	}

	// feedback: .gate/outbox → [.siren/inbox, .expedition/inbox]
	fb, ok := routeMap["design-feedback"]
	if !ok {
		t.Fatal("missing route for kind=design-feedback")
	}
	if fb.From != ".gate/outbox" {
		t.Errorf("feedback.from = %q, want %q", fb.From, ".gate/outbox")
	}
	sort.Strings(fb.To)
	if len(fb.To) != 2 {
		t.Fatalf("feedback.to = %v, want 2 targets", fb.To)
	}
	if fb.To[0] != ".expedition/inbox" || fb.To[1] != ".siren/inbox" {
		t.Errorf("feedback.to = %v, want [.expedition/inbox .siren/inbox]", fb.To)
	}
}

func TestDeriveRoutes_NoEndpoints(t *testing.T) {
	routes := domain.DeriveRoutes(nil)
	if len(routes) != 0 {
		t.Errorf("want 0 routes, got %d", len(routes))
	}
}

func TestDeriveRoutes_OrphanedProducer(t *testing.T) {
	// A kind is produced but nobody consumes it
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"specification"}, Consumes: nil},
	}
	routes := domain.DeriveRoutes(endpoints)
	// No routes should be derived (no consumer)
	if len(routes) != 0 {
		t.Errorf("want 0 routes for orphaned producer, got %d", len(routes))
	}
}

func TestDetectOrphans(t *testing.T) {
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
		{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
		// feedback is consumed by .siren but nobody produces it
		// report is produced by .expedition but nobody consumes it
	}

	orphaned := domain.DetectOrphans(endpoints)

	if len(orphaned.UnconsumedKinds) != 1 || orphaned.UnconsumedKinds[0] != "report" {
		t.Errorf("unconsumed = %v, want [report]", orphaned.UnconsumedKinds)
	}
	if len(orphaned.UnproducedKinds) != 1 || orphaned.UnproducedKinds[0] != "design-feedback" {
		t.Errorf("unproduced = %v, want [design-feedback]", orphaned.UnproducedKinds)
	}
}

func TestDetectOrphans_PerRepoScope(t *testing.T) {
	// given — repo A produces "specification", repo B consumes "specification"
	// Since routing is same_repository, no route can connect them.
	// DetectOrphans should report "specification" as BOTH unconsumed (in A)
	// and unproduced (in B), not silently suppress the warning.
	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: "/repo-a",
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: nil},
				},
			},
			{
				Path: "/repo-b",
				Endpoints: []domain.EndpointConfig{
					{Dir: ".expedition", Produces: nil, Consumes: []string{"specification"}},
				},
			},
		},
	}

	// when — detect orphans respecting per-repo scope
	orphans := domain.DetectOrphansPerRepo(cfg)

	// then — specification should be flagged in BOTH directions
	if len(orphans.UnconsumedKinds) != 1 || orphans.UnconsumedKinds[0] != "specification" {
		t.Errorf("unconsumed = %v, want [specification] (produced in repo A, no consumer in same repo)", orphans.UnconsumedKinds)
	}
	if len(orphans.UnproducedKinds) != 1 || orphans.UnproducedKinds[0] != "specification" {
		t.Errorf("unproduced = %v, want [specification] (consumed in repo B, no producer in same repo)", orphans.UnproducedKinds)
	}
}

func TestDetectOrphans_PerRepo_NoFalsePositivesSingleRepo(t *testing.T) {
	// given — single repo where produces/consumes match perfectly
	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: "/repo",
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification", "design-feedback"}},
					{Dir: ".gate", Produces: []string{"design-feedback"}, Consumes: []string{"report"}},
				},
			},
		},
	}

	// when
	orphans := domain.DetectOrphansPerRepo(cfg)

	// then — no orphans
	if len(orphans.UnconsumedKinds) != 0 {
		t.Errorf("unconsumed = %v, want none", orphans.UnconsumedKinds)
	}
	if len(orphans.UnproducedKinds) != 0 {
		t.Errorf("unproduced = %v, want none", orphans.UnproducedKinds)
	}
}

func TestDetectOrphans_SelfOnlyConsumer(t *testing.T) {
	// given — an endpoint that both produces and consumes the same kind,
	// and no other endpoint consumes it. DeriveRoutes will filter out
	// self-delivery, so this kind has no route. DetectOrphans must flag it.
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"internal"}, Consumes: []string{"internal"}},
	}

	// when
	orphans := domain.DetectOrphans(endpoints)

	// then — "internal" should be unconsumed (self-only consumer is effectively no consumer)
	if len(orphans.UnconsumedKinds) != 1 || orphans.UnconsumedKinds[0] != "internal" {
		t.Errorf("unconsumed = %v, want [internal] (self-only consumer should be flagged)", orphans.UnconsumedKinds)
	}
}

func TestDetectOrphans_SelfConsumerWithExternalConsumer(t *testing.T) {
	// given — endpoint produces and consumes the same kind, but another endpoint
	// also consumes it. A route exists (producer → other consumer). NOT orphaned.
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"data"}, Consumes: []string{"data"}},
		{Dir: ".expedition", Produces: nil, Consumes: []string{"data"}},
	}

	// when
	orphans := domain.DetectOrphans(endpoints)

	// then — "data" should NOT be unconsumed (external consumer .expedition exists)
	if len(orphans.UnconsumedKinds) != 0 {
		t.Errorf("unconsumed = %v, want none (external consumer exists)", orphans.UnconsumedKinds)
	}
}

func TestDeriveRoutes_ExpeditionProducesFeedback(t *testing.T) {
	// given — expedition produces both report AND feedback (escalation d-mail)
	// This matches the updated SKILL.md where paintress declares
	// produces: [report, feedback] for escalation scenarios.
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"design-feedback"}},
		{Dir: ".expedition", Produces: []string{"report", "design-feedback"}, Consumes: []string{"specification", "design-feedback"}},
		{Dir: ".gate", Produces: []string{"design-feedback"}, Consumes: []string{"report"}},
	}

	// when
	routes := domain.DeriveRoutes(endpoints)

	// then — should have 4 routes now:
	// specification: .siren → .expedition
	// report: .expedition → .gate
	// feedback: .gate → [.siren, .expedition]
	// feedback: .expedition → .siren  (NEW: escalation feedback)
	feedbackFromExpedition := false
	for _, r := range routes {
		if r.Kind == "design-feedback" && r.From == ".expedition/outbox" {
			feedbackFromExpedition = true
			// expedition's feedback should go to .siren (the only OTHER consumer)
			// .expedition itself consumes feedback too, but self-delivery is filtered
			if len(r.To) != 1 || r.To[0] != ".siren/inbox" {
				t.Errorf("feedback from .expedition: to = %v, want [.siren/inbox]", r.To)
			}
		}
	}
	if !feedbackFromExpedition {
		t.Error("expected a feedback route from .expedition/outbox (escalation d-mail)")
	}
}

func TestDeriveRoutes_SameKindMultipleProducers(t *testing.T) {
	// Two endpoints produce the same kind — each gets its own route
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"notification"}, Consumes: nil},
		{Dir: ".expedition", Produces: []string{"notification"}, Consumes: nil},
		{Dir: ".gate", Produces: nil, Consumes: []string{"notification"}},
	}

	routes := domain.DeriveRoutes(endpoints)
	if len(routes) != 2 {
		t.Fatalf("want 2 routes (one per producer), got %d: %+v", len(routes), routes)
	}

	// Both should route to .gate/inbox
	for _, r := range routes {
		if r.Kind != "notification" {
			t.Errorf("route.kind = %q, want notification", r.Kind)
		}
		if len(r.To) != 1 || r.To[0] != ".gate/inbox" {
			t.Errorf("route.to = %v, want [.gate/inbox]", r.To)
		}
	}
}

func TestDeriveRoutes_StallEscalation(t *testing.T) {
	// given — sightjack produces stall-escalation, amadeus consumes it
	endpoints := []domain.Endpoint{
		{Dir: ".siren", Produces: []string{"specification", "report", "stall-escalation"}, Consumes: []string{"design-feedback"}},
		{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
		{Dir: ".gate", Produces: []string{"design-feedback"}, Consumes: []string{"report", "stall-escalation"}},
	}

	// when
	routes := domain.DeriveRoutes(endpoints)

	// then — stall-escalation route: .siren/outbox → [.gate/inbox]
	routeMap := make(map[string]domain.Route)
	for _, r := range routes {
		routeMap[r.Kind] = r
	}

	stall, ok := routeMap["stall-escalation"]
	if !ok {
		t.Fatal("missing route for kind=stall-escalation — routing declaration mismatch")
	}
	if stall.From != ".siren/outbox" {
		t.Errorf("stall-escalation.from = %q, want %q", stall.From, ".siren/outbox")
	}
	if len(stall.To) != 1 || stall.To[0] != ".gate/inbox" {
		t.Errorf("stall-escalation.to = %v, want [.gate/inbox]", stall.To)
	}
}
