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
			name: "valid feedback dmail",
			content: `---
dmail-schema-version: "1"
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
kind: feedback
description: "Feedback with metadata"
metadata:
  created_at: "2026-02-22"
  convergence_for: "auth-module"
---
`,
			want: "feedback",
		},
		{
			name: "dmail with metadata produces as string",
			content: `---
dmail-schema-version: "1"
name: feedback-str
kind: feedback
description: "Metadata produces is a string not array"
metadata:
  produces: "some-tool-specific-value"
---
`,
			want: "feedback",
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
kind: feedback
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
	if got != "feedback" {
		t.Errorf("kind = %q, want %q", got, "feedback")
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
		{"ci-result", false},
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
