package session_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

func TestLoadSaveDeliveryFilter_Roundtrip(t *testing.T) {
	// given — a BF with some entries, saved to disk
	stateDir := t.TempDir()
	bf := domain.NewBloomFilter(1000, 0.01)
	bf.Add("/outbox/spec-a.md")
	bf.Add("/outbox/spec-b.md")

	// when — save then load
	if err := session.SaveDeliveryFilter(stateDir, bf); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := session.LoadDeliveryFilter(stateDir)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.MayContain("/outbox/spec-a.md") {
		t.Error("loaded filter lost spec-a")
	}
	if !loaded.MayContain("/outbox/spec-b.md") {
		t.Error("loaded filter lost spec-b")
	}
	if loaded.MayContain("/outbox/spec-c.md") {
		t.Error("loaded filter has false positive for spec-c (check hash)")
	}
}

func TestLoadDeliveryFilter_NoFile_ReturnsNil(t *testing.T) {
	// given — empty state dir with no bloom filter file
	stateDir := t.TempDir()

	// when
	bf, err := session.LoadDeliveryFilter(stateDir)

	// then — no error, nil filter
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if bf != nil {
		t.Error("expected nil filter when no file exists")
	}
}

func TestScanAndDeliver_SkipsAlreadyDelivered(t *testing.T) {
	// given — outbox with two D-Mail files
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailA := `---
dmail-schema-version: "1"
name: spec-a
kind: specification
description: "Already delivered"
---
`
	dmailB := `---
dmail-schema-version: "1"
name: spec-b
kind: specification
description: "New one"
---
`
	pathA := filepath.Join(outbox, "spec-a.md")
	pathB := filepath.Join(outbox, "spec-b.md")
	if err := os.WriteFile(pathA, []byte(dmailA), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte(dmailB), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	// BF already contains pathA — should be skipped
	bf := domain.NewBloomFilter(1000, 0.01)
	bf.Add(pathA)

	// when
	results, errs := session.ScanAndDeliver(context.Background(), outbox, routes, repoDir, &domain.NopLogger{}, ds, nil, bf)

	// then — only spec-b should be delivered
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Kind != "specification" {
		t.Errorf("kind = %q, want specification", results[0].Kind)
	}
	if filepath.Base(results[0].SourcePath) != "spec-b.md" {
		t.Errorf("delivered %s, want spec-b.md", filepath.Base(results[0].SourcePath))
	}

	// spec-a should still be in outbox (skipped, not processed)
	if _, err := os.Stat(pathA); err != nil {
		t.Errorf("spec-a should still exist in outbox: %v", err)
	}
}

func TestScanAndDeliver_NilBloomFilterDeliversAll(t *testing.T) {
	// given — outbox with one D-Mail, no BF
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmail := `---
dmail-schema-version: "1"
name: spec-all
kind: specification
description: "Should deliver"
---
`
	dmailPath := filepath.Join(outbox, "spec-all.md")
	if err := os.WriteFile(dmailPath, []byte(dmail), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	// when — nil BF means no dedup
	results, errs := session.ScanAndDeliver(context.Background(), outbox, routes, repoDir, &domain.NopLogger{}, ds, nil, nil)

	// then — should deliver normally
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
}
