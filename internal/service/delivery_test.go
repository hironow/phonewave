package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	phonewave "github.com/hironow/phonewave"
)

func TestDeliver_SingleTarget(t *testing.T) {
	// given — a repo with outbox and inbox
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a D-Mail to outbox
	dmailContent := `---
dmail-schema-version: "1"
name: spec-001
kind: specification
description: "Test spec"
---

# Test Specification
`
	dmailPath := filepath.Join(outbox, "spec-001.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Route table
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	result, err := Deliver(context.Background(), dmailPath, routes)

	// then
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if result.Kind != "specification" {
		t.Errorf("kind = %q, want specification", result.Kind)
	}
	if len(result.DeliveredTo) != 1 {
		t.Fatalf("delivered to %d targets, want 1", len(result.DeliveredTo))
	}

	// D-Mail should exist in inbox
	deliveredPath := filepath.Join(inbox, "spec-001.md")
	if _, err := os.Stat(deliveredPath); os.IsNotExist(err) {
		t.Error("D-Mail not found in inbox")
	}

	// D-Mail should be removed from outbox
	if _, err := os.Stat(dmailPath); !os.IsNotExist(err) {
		t.Error("D-Mail should be removed from outbox after delivery")
	}
}

func TestDeliver_MultipleTargets(t *testing.T) {
	// given — feedback goes to two inboxes
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, ".siren", "inbox")
	inbox2 := filepath.Join(repoDir, ".expedition", "inbox")
	for _, d := range []string{outbox, inbox1, inbox2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: feedback-042
kind: feedback
description: "Corrective feedback"
---

# Feedback
`
	dmailPath := filepath.Join(outbox, "feedback-042.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}

	// when
	result, err := Deliver(context.Background(), dmailPath, routes)

	// then
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(result.DeliveredTo) != 2 {
		t.Errorf("delivered to %d targets, want 2", len(result.DeliveredTo))
	}

	// Both inboxes should have the file
	for _, inbox := range []string{inbox1, inbox2} {
		if _, err := os.Stat(filepath.Join(inbox, "feedback-042.md")); os.IsNotExist(err) {
			t.Errorf("D-Mail not found in %s", inbox)
		}
	}

	// Source removed
	if _, err := os.Stat(dmailPath); !os.IsNotExist(err) {
		t.Error("source should be removed after delivery")
	}
}

func TestDeliver_UnknownKind(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}

	dmailContent := `---
dmail-schema-version: "1"
name: unknown-001
kind: unknown_type
---
`
	dmailPath := filepath.Join(outbox, "unknown-001.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty routes — no route for "unknown_type"
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{"/tmp/nope"}},
	}

	_, err := Deliver(context.Background(), dmailPath, routes)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
