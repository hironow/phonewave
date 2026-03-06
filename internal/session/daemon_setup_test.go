package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestResolveRoutes(t *testing.T) {
	// given — a config with one repo and routes using relative paths
	repoDir := t.TempDir()

	// Create outbox and inbox directories
	siren := filepath.Join(repoDir, ".siren")
	expedition := filepath.Join(repoDir, ".expedition")
	gate := filepath.Join(repoDir, ".gate")
	for _, dir := range []string{
		filepath.Join(siren, "outbox"),
		filepath.Join(siren, "inbox"),
		filepath.Join(expedition, "outbox"),
		filepath.Join(expedition, "inbox"),
		filepath.Join(gate, "outbox"),
		filepath.Join(gate, "inbox"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification", "feedback"}},
					{Dir: ".gate", Produces: []string{"feedback"}, Consumes: []string{"report"}},
				},
			},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
			{Kind: "report", From: ".expedition/outbox", To: []string{".gate/inbox"}, Scope: "same_repository"},
			{Kind: "feedback", From: ".gate/outbox", To: []string{".siren/inbox", ".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	// when
	resolved, err := session.ResolveRoutes(cfg)

	// then
	if err != nil {
		t.Fatalf("ResolveRoutes: %v", err)
	}
	if len(resolved) != 3 {
		t.Fatalf("resolved routes = %d, want 3", len(resolved))
	}

	// Verify absolute paths
	for _, r := range resolved {
		if !filepath.IsAbs(r.FromOutbox) {
			t.Errorf("FromOutbox %q is not absolute", r.FromOutbox)
		}
		for _, inbox := range r.ToInboxes {
			if !filepath.IsAbs(inbox) {
				t.Errorf("ToInbox %q is not absolute", inbox)
			}
		}
	}

	// Check specification route
	specRoute := findResolvedRoute(resolved, "specification")
	if specRoute == nil {
		t.Fatal("specification route not found")
	}
	if specRoute.FromOutbox != filepath.Join(repoDir, ".siren", "outbox") {
		t.Errorf("spec FromOutbox = %q, want %q", specRoute.FromOutbox, filepath.Join(repoDir, ".siren", "outbox"))
	}
	if len(specRoute.ToInboxes) != 1 || specRoute.ToInboxes[0] != filepath.Join(repoDir, ".expedition", "inbox") {
		t.Errorf("spec ToInboxes = %v", specRoute.ToInboxes)
	}

	// Check feedback route (multiple targets)
	fbRoute := findResolvedRoute(resolved, "feedback")
	if fbRoute == nil {
		t.Fatal("feedback route not found")
	}
	if len(fbRoute.ToInboxes) != 2 {
		t.Errorf("feedback ToInboxes = %d, want 2", len(fbRoute.ToInboxes))
	}
}

func TestResolveRoutes_MultiRepoSameEndpoint(t *testing.T) {
	// given — two repos, both containing .siren
	// Repo A: .siren produces "specification"
	// Repo B: .siren produces "alert"
	repoA := t.TempDir()
	repoB := t.TempDir()

	for _, dir := range []string{
		filepath.Join(repoA, ".siren", "outbox"),
		filepath.Join(repoA, ".expedition", "inbox"),
		filepath.Join(repoB, ".siren", "outbox"),
		filepath.Join(repoB, ".gate", "inbox"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoA,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: nil},
					{Dir: ".expedition", Produces: nil, Consumes: []string{"specification"}},
				},
			},
			{
				Path: repoB,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"alert"}, Consumes: nil},
					{Dir: ".gate", Produces: nil, Consumes: []string{"alert"}},
				},
			},
		},
	}
	cfg.UpdateRoutes()

	// when
	resolved, err := session.ResolveRoutes(cfg)

	// then
	if err != nil {
		t.Fatalf("ResolveRoutes: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved routes = %d, want 2", len(resolved))
	}

	// specification route must resolve to repo A paths
	specRoute := findResolvedRoute(resolved, "specification")
	if specRoute == nil {
		t.Fatal("specification route not found")
	}
	wantSpecFrom := filepath.Join(repoA, ".siren", "outbox")
	if specRoute.FromOutbox != wantSpecFrom {
		t.Errorf("specification FromOutbox = %q, want %q (repo A)", specRoute.FromOutbox, wantSpecFrom)
	}

	// alert route must resolve to repo B paths, NOT repo A
	alertRoute := findResolvedRoute(resolved, "alert")
	if alertRoute == nil {
		t.Fatal("alert route not found")
	}
	wantAlertFrom := filepath.Join(repoB, ".siren", "outbox")
	if alertRoute.FromOutbox != wantAlertFrom {
		t.Errorf("alert FromOutbox = %q, want %q (repo B)", alertRoute.FromOutbox, wantAlertFrom)
	}
	wantAlertTo := filepath.Join(repoB, ".gate", "inbox")
	if len(alertRoute.ToInboxes) != 1 || alertRoute.ToInboxes[0] != wantAlertTo {
		t.Errorf("alert ToInboxes = %v, want [%s] (repo B)", alertRoute.ToInboxes, wantAlertTo)
	}
}

func findResolvedRoute(routes []domain.ResolvedRoute, kind string) *domain.ResolvedRoute {
	for i := range routes {
		if routes[i].Kind == kind {
			return &routes[i]
		}
	}
	return nil
}

func TestResolveRoutes_CollectsOutboxDirs(t *testing.T) {
	// given — same setup
	repoDir := t.TempDir()
	for _, dir := range []string{
		filepath.Join(repoDir, ".siren", "outbox"),
		filepath.Join(repoDir, ".expedition", "outbox"),
		filepath.Join(repoDir, ".gate", "outbox"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &domain.Config{
		Repositories: []domain.RepoConfig{
			{
				Path: repoDir,
				Endpoints: []domain.EndpointConfig{
					{Dir: ".siren", Produces: []string{"specification"}, Consumes: []string{"feedback"}},
					{Dir: ".expedition", Produces: []string{"report"}, Consumes: []string{"specification"}},
					{Dir: ".gate", Produces: []string{"feedback"}, Consumes: []string{"report"}},
				},
			},
		},
		Routes: []domain.RouteConfig{
			{Kind: "specification", From: ".siren/outbox", To: []string{".expedition/inbox"}, Scope: "same_repository"},
		},
	}

	// when
	resolved, err := session.ResolveRoutes(cfg)

	// then
	if err != nil {
		t.Fatalf("ResolveRoutes: %v", err)
	}

	// OutboxDirs should contain all outbox directories from endpoints that produce
	outboxDirs := session.CollectOutboxDirs(cfg)
	if len(outboxDirs) != 3 {
		t.Errorf("outbox dirs = %d, want 3", len(outboxDirs))
	}
	_ = resolved
}
