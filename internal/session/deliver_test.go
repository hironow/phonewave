package session

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

func TestDeliver_FileVanished(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Reference a non-existent file
	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when — try to deliver a file that doesn't exist
	_, err := Deliver(context.Background(), filepath.Join(outbox, "ghost.md"), routes)

	// then — should return error, not panic
	if err == nil {
		t.Fatal("expected error for vanished file")
	}
}

func TestDeliver_OverwriteExistingInInbox(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Pre-existing file in inbox with different content
	oldContent := []byte("old version")
	if err := os.WriteFile(filepath.Join(inbox, "spec-dup.md"), oldContent, 0644); err != nil {
		t.Fatal(err)
	}

	// New D-Mail with same name
	newContent := `---
dmail-schema-version: "1"
name: spec-dup
kind: specification
description: "New version"
---

# Updated specification
`
	if err := os.WriteFile(filepath.Join(outbox, "spec-dup.md"), []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	// when
	result, err := Deliver(context.Background(), filepath.Join(outbox, "spec-dup.md"), routes)

	// then — should succeed (atomic rename overwrites)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if result.Kind != "specification" {
		t.Errorf("kind = %q, want specification", result.Kind)
	}

	// Inbox should have the NEW content
	data, err := os.ReadFile(filepath.Join(inbox, "spec-dup.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != newContent {
		t.Error("inbox file should contain new content (overwritten)")
	}
}

func TestDeliver_MissingInboxDir(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	if err := os.MkdirAll(outbox, 0755); err != nil {
		t.Fatal(err)
	}

	// DO NOT create the inbox directory
	nonExistentInbox := filepath.Join(repoDir, ".expedition", "inbox")

	dmailContent := `---
dmail-schema-version: "1"
name: spec-noinbox
kind: specification
description: "No inbox target"
---
`
	dmailPath := filepath.Join(outbox, "spec-noinbox.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{nonExistentInbox}},
	}

	// when
	_, err := Deliver(context.Background(), dmailPath, routes)

	// then — should return error (can't create temp file in nonexistent dir)
	if err == nil {
		t.Fatal("expected error when inbox directory doesn't exist")
	}

	// Source should NOT be removed (delivery failed)
	if _, err := os.Stat(dmailPath); os.IsNotExist(err) {
		t.Error("source should still exist when delivery fails")
	}
}

func TestDeliver_PartialFailure_RollsBackDeliveredInboxes(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, ".siren", "inbox")
	// inbox2 does NOT exist — will cause partial failure
	inbox2 := filepath.Join(repoDir, ".expedition", "inbox-nonexistent")

	for _, dir := range []string{outbox, inbox1} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: fb-partial
kind: feedback
description: "Partial failure test"
---
`
	dmailPath := filepath.Join(outbox, "fb-partial.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []phonewave.ResolvedRoute{
		{Kind: "feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}

	// when
	_, err := Deliver(context.Background(), dmailPath, routes)

	// then — should return error (partial failure)
	if err == nil {
		t.Fatal("expected error for partial failure")
	}

	// inbox1 should be cleaned up (rolled back) to prevent duplicates on retry
	if _, err := os.Stat(filepath.Join(inbox1, "fb-partial.md")); !os.IsNotExist(err) {
		t.Error("inbox1 should be rolled back on partial delivery failure to prevent duplicates on retry")
	}

	// Source should still exist (delivery failed)
	if _, err := os.Stat(dmailPath); os.IsNotExist(err) {
		t.Error("source should still exist after delivery failure")
	}
}
