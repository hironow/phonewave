package phonewave

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractDMailKind(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name: "valid feedback dmail",
			content: `---
name: feedback-001
kind: feedback
description: "ADR-003 violation detected"
---

# ADR-003 Violation
`,
			want: "feedback",
		},
		{
			name: "valid specification dmail",
			content: `---
name: spec-auth
kind: specification
description: "Auth session management"
issues:
  - MY-42
---

# Auth Session Spec
`,
			want: "specification",
		},
		{
			name: "valid report dmail",
			content: `---
name: report-001
kind: report
description: "Implementation report"
---
`,
			want: "report",
		},
		{
			name: "valid convergence dmail",
			content: `---
name: conv-001
kind: convergence
description: "Convergence alert"
---
`,
			want: "convergence",
		},
		{
			name:    "no frontmatter",
			content: "# Just markdown",
			wantErr: true,
		},
		{
			name: "missing kind field",
			content: `---
name: no-kind
description: "Missing kind"
---
`,
			wantErr: true,
		},
		{
			name: "invalid kind value",
			content: `---
name: bad-kind
kind: invalid_type
description: "Not a valid kind"
---
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDMailKind([]byte(tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("kind = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateKind(t *testing.T) {
	tests := []struct {
		kind    string
		wantErr bool
	}{
		{"specification", false},
		{"report", false},
		{"feedback", false},
		{"convergence", false},
		{"", true},
		{"unknown", true},
		{"SPECIFICATION", true},
		{"spec", true},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			err := ValidateKind(tt.kind)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateKind(%q) = nil, want error", tt.kind)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateKind(%q) = %v, want nil", tt.kind, err)
			}
		})
	}
}

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
	routes := []ResolvedRoute{
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

	routes := []ResolvedRoute{
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
name: unknown-001
kind: unknown_type
---
`
	dmailPath := filepath.Join(outbox, "unknown-001.md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty routes — no route for "unknown_type"
	routes := []ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{"/tmp/nope"}},
	}

	_, err := Deliver(context.Background(), dmailPath, routes)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
