package domain_test

import (
	"slices"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestCorrectionMetadataApplyRoundTrip(t *testing.T) {
	meta := domain.CorrectionMetadata{
		FailureType:      domain.FailureTypeRoutingFailure,
		Severity:         domain.SeverityMedium,
		SecondaryType:    "delivery",
		TargetAgent:      "paintress",
		RoutingMode:      domain.RoutingModeRetry,
		RecurrenceCount:  2,
		CorrectiveAction: "retry",
		RetryAllowed:     domain.BoolPtr(true),
		EscalationReason: "recurrence-threshold",
		CorrelationID:    "corr-1",
		TraceID:          "trace-1",
		Outcome:          domain.ImprovementOutcomePending,
	}

	applied := meta.Apply(map[string]string{"existing": "ok"})
	got := domain.CorrectionMetadataFromMap(applied)

	if got.FailureType != meta.FailureType {
		t.Fatalf("FailureType = %q, want %q", got.FailureType, meta.FailureType)
	}
	if got.Severity != meta.Severity {
		t.Fatalf("Severity = %q, want %q", got.Severity, meta.Severity)
	}
	if got.TargetAgent != "paintress" {
		t.Fatalf("TargetAgent = %q, want paintress", got.TargetAgent)
	}
	if got.RoutingMode != domain.RoutingModeRetry {
		t.Fatalf("RoutingMode = %q, want %q", got.RoutingMode, domain.RoutingModeRetry)
	}
	if got.RecurrenceCount != 2 {
		t.Fatalf("RecurrenceCount = %d, want 2", got.RecurrenceCount)
	}
	if got.RetryAllowed == nil || !*got.RetryAllowed {
		t.Fatal("RetryAllowed = nil/false, want true")
	}
	if got.EscalationReason != "recurrence-threshold" {
		t.Fatalf("EscalationReason = %q, want recurrence-threshold", got.EscalationReason)
	}
	if applied[domain.MetadataImprovementSchemaVersion] != domain.ImprovementSchemaVersion {
		t.Fatalf("schema version = %q, want %q", applied[domain.MetadataImprovementSchemaVersion], domain.ImprovementSchemaVersion)
	}
	if applied[domain.MetadataSeverity] != string(domain.SeverityMedium) {
		t.Fatalf("severity = %q, want %q", applied[domain.MetadataSeverity], domain.SeverityMedium)
	}
}

func TestCorrectionMetadataForwardForRecheck(t *testing.T) {
	meta := domain.CorrectionMetadata{
		FailureType:      domain.FailureTypeRoutingFailure,
		TargetAgent:      "paintress",
		CorrelationID:    "corr-1",
		CorrectiveAction: "retry",
	}

	got := meta.ForwardForRecheck()

	if got.TargetAgent != "" {
		t.Fatalf("TargetAgent = %q, want empty", got.TargetAgent)
	}
	if got.RoutingMode != "" {
		t.Fatalf("RoutingMode = %q, want empty", got.RoutingMode)
	}
	if got.Outcome != domain.ImprovementOutcomePending {
		t.Fatalf("Outcome = %q, want %q", got.Outcome, domain.ImprovementOutcomePending)
	}
	if got.SchemaVersion != domain.ImprovementSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", got.SchemaVersion, domain.ImprovementSchemaVersion)
	}
	if got.RetryAllowed != nil {
		t.Fatalf("RetryAllowed = %v, want nil", *got.RetryAllowed)
	}
}

func TestCorrectionMetadataFromMap_LegacyV1WithoutSchemaVersion(t *testing.T) {
	got := domain.CorrectionMetadataFromMap(map[string]string{
		domain.MetadataFailureType: "routing_failure",
		domain.MetadataSeverity:    "HIGH",
		domain.MetadataOutcome:     "FAILED_AGAIN",
	})

	if !got.IsImprovement() {
		t.Fatal("IsImprovement = false, want true")
	}
	if got.ConsumerSchemaVersion() != domain.ImprovementSchemaVersion {
		t.Fatalf("ConsumerSchemaVersion = %q, want %q", got.ConsumerSchemaVersion(), domain.ImprovementSchemaVersion)
	}
	if got.Severity != domain.SeverityHigh {
		t.Fatalf("Severity = %q, want %q", got.Severity, domain.SeverityHigh)
	}
	if got.Outcome != domain.ImprovementOutcomeFailedAgain {
		t.Fatalf("Outcome = %q, want %q", got.Outcome, domain.ImprovementOutcomeFailedAgain)
	}
	if !got.HasSupportedVocabulary() {
		t.Fatal("HasSupportedVocabulary = false, want true")
	}
}

func TestCorrectionMetadataHasSupportedVocabulary_RejectsUnknownOutcome(t *testing.T) {
	meta := domain.CorrectionMetadata{
		FailureType: domain.FailureTypeRoutingFailure,
		Severity:    domain.SeverityMedium,
		Outcome:     domain.ImprovementOutcome("not-real"),
	}

	if meta.HasSupportedVocabulary() {
		t.Fatal("HasSupportedVocabulary = true, want false")
	}
}

func TestFilterInboxesByTargetAgent(t *testing.T) {
	inboxes := []string{
		"/tmp/run/sightjack/.siren/inbox",
		"/tmp/run/paintress/.expedition/inbox",
	}
	got := domain.FilterInboxesByTargetAgent(inboxes, "paintress")
	if !slices.Equal(got, []string{"/tmp/run/paintress/.expedition/inbox"}) {
		t.Fatalf("filtered inboxes = %v", got)
	}
}

func TestPreferredImprovementTargetAgent(t *testing.T) {
	tests := []struct {
		name string
		kind string
		meta domain.CorrectionMetadata
		want string
	}{
		{
			name: "explicit target agent wins",
			kind: "design-feedback",
			meta: domain.CorrectionMetadata{
				TargetAgent: "paintress",
			},
			want: "paintress",
		},
		{
			name: "retry mode falls back to design owner",
			kind: "design-feedback",
			meta: domain.CorrectionMetadata{
				RoutingMode: domain.RoutingModeRetry,
			},
			want: "sightjack",
		},
		{
			name: "escalated feedback does not synthesize target",
			kind: "implementation-feedback",
			meta: domain.CorrectionMetadata{
				Outcome: domain.ImprovementOutcomeEscalated,
			},
			want: "",
		},
		{
			name: "retry disabled feedback does not synthesize target",
			kind: "implementation-feedback",
			meta: domain.CorrectionMetadata{
				RetryAllowed: domain.BoolPtr(false),
			},
			want: "",
		},
		{
			name: "reroute without explicit target does not guess",
			kind: "implementation-feedback",
			meta: domain.CorrectionMetadata{
				RoutingMode: domain.RoutingModeReroute,
			},
			want: "",
		},
		{
			name: "non improvement leaves routing broad",
			kind: "design-feedback",
			meta: domain.CorrectionMetadata{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.PreferredImprovementTargetAgent(tt.kind, tt.meta)
			if got != tt.want {
				t.Fatalf("PreferredImprovementTargetAgent(%q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestFilterInboxesByTargets(t *testing.T) {
	inboxes := []string{
		"/tmp/run/sightjack/.siren/inbox",
		"/tmp/run/paintress/.expedition/inbox",
	}
	got := domain.FilterInboxesByTargets(inboxes, []string{"auth/session.go", "paintress"})
	if !slices.Equal(got, []string{"/tmp/run/paintress/.expedition/inbox"}) {
		t.Fatalf("filtered inboxes = %v", got)
	}
}
