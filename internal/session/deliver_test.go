package session_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
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
	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	// when
	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)

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
	if _, err := os.Stat(deliveredPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("D-Mail not found in inbox")
	}

	// D-Mail should be removed from outbox
	if _, err := os.Stat(dmailPath); !errors.Is(err, fs.ErrNotExist) {
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
kind: design-feedback
description: "Corrective feedback"
---

# Feedback
`
	dmailPath := filepath.Join(outbox, "feedback-042.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "design-feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}

	ds := newTestDeliveryStore(t)

	// when
	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)

	// then
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(result.DeliveredTo) != 2 {
		t.Errorf("delivered to %d targets, want 2", len(result.DeliveredTo))
	}

	// Both inboxes should have the file
	for _, inbox := range []string{inbox1, inbox2} {
		if _, err := os.Stat(filepath.Join(inbox, "feedback-042.md")); errors.Is(err, fs.ErrNotExist) {
			t.Errorf("D-Mail not found in %s", inbox)
		}
	}

	// Source removed
	if _, err := os.Stat(dmailPath); !errors.Is(err, fs.ErrNotExist) {
		t.Error("source should be removed after delivery")
	}
}

func TestDeliver_TargetAgentMetadataNarrowsTargets(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, "amadeus", ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, "sightjack", ".siren", "inbox")
	inbox2 := filepath.Join(repoDir, "paintress", ".expedition", "inbox")
	for _, d := range []string{outbox, inbox1, inbox2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: feedback-043
kind: design-feedback
description: "Corrective feedback"
metadata:
  target_agent: paintress
  failure_type: execution_failure
---

# Feedback
`
	dmailPath := filepath.Join(outbox, "feedback-043.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "design-feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}
	ds := newTestDeliveryStore(t)

	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(result.DeliveredTo) != 1 {
		t.Fatalf("delivered to %d targets, want 1", len(result.DeliveredTo))
	}
	targetPath := filepath.Join(inbox2, "feedback-043.md")
	if result.DeliveredTo[0] != targetPath {
		t.Fatalf("delivered to %q, want %q", result.DeliveredTo[0], targetPath)
	}
	if _, err := os.Stat(filepath.Join(inbox1, "feedback-043.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("unexpected delivery to non-target inbox")
	}
	if _, err := os.Stat(filepath.Join(inbox2, "feedback-043.md")); err != nil {
		t.Fatalf("target inbox missing delivery: %v", err)
	}
}

func TestDeliver_TargetsFrontmatterNarrowsTargets(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, "amadeus", ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, "sightjack", ".siren", "inbox")
	inbox2 := filepath.Join(repoDir, "paintress", ".expedition", "inbox")
	for _, d := range []string{outbox, inbox1, inbox2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: feedback-044
kind: design-feedback
description: "Corrective feedback"
targets:
  - auth/session.go
  - paintress
metadata:
  failure_type: execution_failure
---

# Feedback
`
	dmailPath := filepath.Join(outbox, "feedback-044.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "design-feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}
	ds := newTestDeliveryStore(t)

	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(result.DeliveredTo) != 1 {
		t.Fatalf("delivered to %d targets, want 1", len(result.DeliveredTo))
	}
	targetPath := filepath.Join(inbox2, "feedback-044.md")
	if result.DeliveredTo[0] != targetPath {
		t.Fatalf("delivered to %q, want %q", result.DeliveredTo[0], targetPath)
	}
	if _, err := os.Stat(filepath.Join(inbox1, "feedback-044.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("unexpected delivery to non-target inbox")
	}
	if _, err := os.Stat(filepath.Join(inbox2, "feedback-044.md")); err != nil {
		t.Fatalf("target inbox missing delivery: %v", err)
	}
}

func TestDeliver_EscalatedImprovementDoesNotSynthesizePreferredTarget(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, "amadeus", ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, "sightjack", ".siren", "inbox")
	inbox2 := filepath.Join(repoDir, "paintress", ".expedition", "inbox")
	for _, d := range []string{outbox, inbox1, inbox2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: feedback-045
kind: design-feedback
description: "Escalated corrective feedback"
metadata:
  improvement_schema_version: "1"
  failure_type: scope_violation
  outcome: escalated
  retry_allowed: "false"
---

# Feedback
`
	dmailPath := filepath.Join(outbox, "feedback-045.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "design-feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}
	ds := newTestDeliveryStore(t)

	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(result.DeliveredTo) != 2 {
		t.Fatalf("delivered to %d targets, want 2", len(result.DeliveredTo))
	}
	for _, inbox := range []string{inbox1, inbox2} {
		if _, err := os.Stat(filepath.Join(inbox, "feedback-045.md")); err != nil {
			t.Fatalf("expected delivery in %s: %v", inbox, err)
		}
	}
}

func TestDeliver_EscalatedImprovementNarrowsToExplicitHandoffOwner(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, "amadeus", ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, "sightjack", ".siren", "inbox")
	inbox2 := filepath.Join(repoDir, "paintress", ".expedition", "inbox")
	for _, d := range []string{outbox, inbox1, inbox2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: feedback-045b
kind: implementation-feedback
description: "Escalated corrective feedback"
metadata:
  improvement_schema_version: "1"
  failure_type: execution_failure
  outcome: escalated
  retry_allowed: "false"
  routing_mode: escalate
  target_agent: paintress
  severity: high
---

# Feedback
`
	dmailPath := filepath.Join(outbox, "feedback-045b.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "implementation-feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}
	ds := newTestDeliveryStore(t)

	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(result.DeliveredTo) != 1 {
		t.Fatalf("delivered to %d targets, want 1", len(result.DeliveredTo))
	}
	targetPath := filepath.Join(inbox2, "feedback-045b.md")
	if result.DeliveredTo[0] != targetPath {
		t.Fatalf("delivered to %q, want %q", result.DeliveredTo[0], targetPath)
	}
	if _, err := os.Stat(filepath.Join(inbox1, "feedback-045b.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("unexpected delivery to non-target inbox")
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("target inbox missing delivery: %v", err)
	}
}

func TestDeliver_TargetsTakePrecedenceOverImprovementTargetAgent(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, "amadeus", ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, "sightjack", ".siren", "inbox")
	inbox2 := filepath.Join(repoDir, "paintress", ".expedition", "inbox")
	for _, d := range []string{outbox, inbox1, inbox2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: feedback-046
kind: design-feedback
description: "Targets win over improvement target"
targets:
  - paintress
metadata:
  improvement_schema_version: "1"
  target_agent: sightjack
  routing_mode: reroute
  failure_type: execution_failure
  outcome: pending
---

# Feedback
`
	dmailPath := filepath.Join(outbox, "feedback-046.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "design-feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}
	ds := newTestDeliveryStore(t)

	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(result.DeliveredTo) != 1 {
		t.Fatalf("delivered to %d targets, want 1", len(result.DeliveredTo))
	}
	targetPath := filepath.Join(inbox2, "feedback-046.md")
	if result.DeliveredTo[0] != targetPath {
		t.Fatalf("delivered to %q, want %q", result.DeliveredTo[0], targetPath)
	}
	if _, err := os.Stat(filepath.Join(inbox1, "feedback-046.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("unexpected delivery to improvement target agent")
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
	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{"/tmp/nope"}},
	}

	ds := newTestDeliveryStore(t)

	_, err := session.Deliver(context.Background(), dmailPath, routes, ds)
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
	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	// when — try to deliver a file that doesn't exist
	_, err := session.Deliver(context.Background(), filepath.Join(outbox, "ghost.md"), routes, ds)

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

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}

	ds := newTestDeliveryStore(t)

	// when
	result, err := session.Deliver(context.Background(), filepath.Join(outbox, "spec-dup.md"), routes, ds)

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

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{nonExistentInbox}},
	}

	ds := newTestDeliveryStore(t)

	// when
	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)

	// then — Stage→Flush: flush failure is NOT returned as error.
	// DeliveryStore retry_count handles re-flush on next delivery.
	if err != nil {
		t.Fatalf("Deliver: unexpected error: %v", err)
	}

	// DeliveredTo should be empty (flush failed for all targets)
	if len(result.DeliveredTo) != 0 {
		t.Errorf("DeliveredTo = %d, want 0", len(result.DeliveredTo))
	}

	// Source should still exist (not all targets flushed)
	if _, err := os.Stat(dmailPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("source should still exist when not all targets flushed")
	}
}

func TestDeliver_PartialFlush_SuccessfulTargetsKept(t *testing.T) {
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".gate", "outbox")
	inbox1 := filepath.Join(repoDir, ".siren", "inbox")
	// inbox2 does NOT exist — will cause partial flush failure
	inbox2 := filepath.Join(repoDir, ".expedition", "inbox-nonexistent")

	for _, dir := range []string{outbox, inbox1} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: fb-partial
kind: design-feedback
description: "Partial failure test"
---
`
	dmailPath := filepath.Join(outbox, "fb-partial.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "design-feedback", FromOutbox: outbox, ToInboxes: []string{inbox1, inbox2}},
	}

	ds := newTestDeliveryStore(t)

	// when
	result, err := session.Deliver(context.Background(), dmailPath, routes, ds)

	// then — Stage→Flush: partial flush failure is NOT returned as error.
	// Successfully flushed targets are kept; failed targets will be retried.
	if err != nil {
		t.Fatalf("Deliver: unexpected error: %v", err)
	}

	// inbox1 should have the file (partial success, no rollback in Stage→Flush)
	if _, err := os.Stat(filepath.Join(inbox1, "fb-partial.md")); errors.Is(err, fs.ErrNotExist) {
		t.Error("inbox1 should have file (partial flush success)")
	}

	// DeliveredTo should have 1 entry (inbox1 only)
	if len(result.DeliveredTo) != 1 {
		t.Errorf("DeliveredTo = %d, want 1", len(result.DeliveredTo))
	}

	// Source should still exist (not all targets flushed)
	if _, err := os.Stat(dmailPath); errors.Is(err, fs.ErrNotExist) {
		t.Error("source should still exist (not all targets flushed)")
	}
}

// --- Dedup strict error handling tests ---

// failDedupStore is a DeliveryDedupStore that returns errors on demand.
type failDedupStore struct {
	hasDeliveredErr  error
	recordErr        error
	hasDeliveredResp bool
}

func (f *failDedupStore) HasDelivered(_ context.Context, _, _ string) (bool, error) {
	if f.hasDeliveredErr != nil {
		return false, f.hasDeliveredErr
	}
	return f.hasDeliveredResp, nil
}

func (f *failDedupStore) RecordDelivery(_ context.Context, _, _ string) error {
	return f.recordErr
}

func (f *failDedupStore) Close() error { return nil }

func TestDeliverData_DedupReadError_ReturnsError(t *testing.T) {
	// given — a D-Mail file and a dedup store that fails on read
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: spec-dedup-read
kind: specification
description: "Dedup read error test"
---
`
	dmailPath := filepath.Join(outbox, "spec-dedup-read.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}
	ds := newTestDeliveryStore(t)
	dedup := &failDedupStore{hasDeliveredErr: errors.New("sqlite: database is locked")}

	// when
	result, err := session.DeliverData(context.Background(), dmailPath, []byte(dmailContent), routes, ds, dedup)

	// then — error returned (fail-closed), source stays in outbox
	if err == nil {
		t.Fatal("expected error from dedup read failure, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on dedup read error, got %+v", result)
	}
	if _, statErr := os.Stat(dmailPath); statErr != nil {
		t.Errorf("source should remain in outbox after dedup read error: %v", statErr)
	}
}

func TestDeliverData_DedupRecordError_SourceStaysNoError(t *testing.T) {
	// given — a D-Mail file and a dedup store that fails on write
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: spec-dedup-write
kind: specification
description: "Dedup write error test"
---
`
	dmailPath := filepath.Join(outbox, "spec-dedup-write.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}
	ds := newTestDeliveryStore(t)
	dedup := &failDedupStore{recordErr: errors.New("sqlite: disk I/O error")}

	// when
	result, err := session.DeliverData(context.Background(), dmailPath, []byte(dmailContent), routes, ds, dedup)

	// then — NO error (delivery succeeded; dedup record failure is not delivery failure)
	if err != nil {
		t.Fatalf("unexpected error: dedup record failure should not be returned as delivery error: %v", err)
	}
	// result should contain the delivered targets (files were already written)
	if result == nil {
		t.Fatal("expected non-nil result with deliveries")
	}
	if len(result.DeliveredTo) == 0 {
		t.Error("expected at least one delivered target")
	}
	// source must still exist (dedup record incomplete → source stays for retry)
	if _, statErr := os.Stat(dmailPath); statErr != nil {
		t.Errorf("source should remain in outbox when dedup record failed: %v", statErr)
	}
	// inbox should have the file (delivery succeeded)
	if _, statErr := os.Stat(filepath.Join(inbox, "spec-dedup-write.md")); statErr != nil {
		t.Errorf("inbox should have file (delivery succeeded): %v", statErr)
	}
}

func TestDeliverData_DedupSuccess_SkipsDuplicate(t *testing.T) {
	// given — a D-Mail file and a dedup store that says already delivered
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	for _, dir := range []string{outbox, inbox} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	dmailContent := `---
dmail-schema-version: "1"
name: spec-already
kind: specification
description: "Already delivered"
---
`
	dmailPath := filepath.Join(outbox, "spec-already.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	routes := []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}
	ds := newTestDeliveryStore(t)
	dedup := &failDedupStore{hasDeliveredResp: true}

	// when
	result, err := session.DeliverData(context.Background(), dmailPath, []byte(dmailContent), routes, ds, dedup)

	// then — no error, no deliveries, source removed
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.DeliveredTo) != 0 {
		t.Errorf("want 0 deliveries for duplicate, got %d", len(result.DeliveredTo))
	}
	if _, statErr := os.Stat(dmailPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Error("source should be removed after full dedup skip")
	}
}
