package domain_test

import (
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestExtractDMailKind(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name: "valid design-feedback dmail",
			content: `---
dmail-schema-version: "1"
name: feedback-001
kind: design-feedback
description: "ADR-003 violation detected"
---

# ADR-003 Violation
`,
			want: "design-feedback",
		},
		{
			name: "valid specification dmail",
			content: `---
dmail-schema-version: "1"
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
dmail-schema-version: "1"
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
dmail-schema-version: "1"
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
dmail-schema-version: "1"
name: no-kind
description: "Missing kind"
---
`,
			wantErr: true,
		},
		{
			name: "dmail with string metadata values",
			content: `---
dmail-schema-version: "1"
name: feedback-meta
kind: design-feedback
description: "Feedback with metadata"
metadata:
  created_at: "2026-02-22"
  convergence_for: "auth-module"
---
`,
			want: "design-feedback",
		},
		{
			name: "dmail with metadata produces as string",
			content: `---
dmail-schema-version: "1"
name: feedback-str
kind: implementation-feedback
description: "Metadata produces is a string not array"
metadata:
  produces: "some-tool-specific-value"
---
`,
			want: "implementation-feedback",
		},
		{
			name: "invalid kind value",
			content: `---
dmail-schema-version: "1"
name: bad-kind
kind: invalid_type
description: "Not a valid kind"
---
`,
			wantErr: true,
		},
		{
			name: "missing dmail-schema-version",
			content: `---
name: no-version
kind: specification
description: "Missing schema version"
---
`,
			wantErr: true,
		},
		{
			name: "unsupported dmail-schema-version",
			content: `---
dmail-schema-version: "2"
name: bad-version
kind: specification
description: "Unsupported schema version"
---
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ExtractDMailKind([]byte(tt.content))
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

func TestValidateKind_CIResult(t *testing.T) {
	// given
	kind := "ci-result"

	// when
	err := domain.ValidateKind(kind)

	// then
	if err != nil {
		t.Errorf("domain.ValidateKind(%q) = %v, want nil", kind, err)
	}
}

func TestExtractDMailKind_WithActionField(t *testing.T) {
	// given
	content := `---
dmail-schema-version: "1"
name: feedback-action-001
kind: design-feedback
description: "Evaluation with retry action"
action: retry
priority: 2
---

Implementation needs revision.
`

	// when
	got, err := domain.ExtractDMailKind([]byte(content))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "design-feedback" {
		t.Errorf("kind = %q, want %q", got, "design-feedback")
	}
}

func TestValidateKind_DesignFeedback(t *testing.T) {
	if err := domain.ValidateKind("design-feedback"); err != nil {
		t.Errorf("expected design-feedback to be valid, got: %v", err)
	}
}

func TestValidateKind_ImplementationFeedback(t *testing.T) {
	if err := domain.ValidateKind("implementation-feedback"); err != nil {
		t.Errorf("expected implementation-feedback to be valid, got: %v", err)
	}
}

func TestValidateKind_OldFeedback_Invalid(t *testing.T) {
	if err := domain.ValidateKind("feedback"); err == nil {
		t.Error("expected feedback to be invalid after kind split")
	}
}

func TestValidateKind(t *testing.T) {
	tests := []struct {
		kind    string
		wantErr bool
	}{
		{"specification", false},
		{"report", false},
		{"design-feedback", false},
		{"implementation-feedback", false},
		{"convergence", false},
		{"ci-result", false},
		{"feedback", true},
		{"", true},
		{"unknown", true},
		{"SPECIFICATION", true},
		{"spec", true},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			err := domain.ValidateKind(tt.kind)
			if tt.wantErr && err == nil {
				t.Errorf("domain.ValidateKind(%q) = nil, want error", tt.kind)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("domain.ValidateKind(%q) = %v, want nil", tt.kind, err)
			}
		})
	}
}
