package policy_test

import (
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/harness/policy"
)

func TestSelectDeliveryInboxes_TargetsTakePrecedence(t *testing.T) {
	inboxes := []string{
		"/repo/sightjack/.siren/inbox",
		"/repo/paintress/.expedition/inbox",
	}

	got := policy.SelectDeliveryInboxes(
		domain.KindDesignFeedback,
		inboxes,
		[]string{"paintress"},
		domain.CorrectionMetadata{TargetAgent: "sightjack"},
	)

	if len(got) != 1 || got[0] != inboxes[1] {
		t.Fatalf("SelectDeliveryInboxes() = %v, want [%q]", got, inboxes[1])
	}
}

func TestSelectDeliveryInboxes_ExplicitEscalatedOwnerNarrowsDelivery(t *testing.T) {
	inboxes := []string{
		"/repo/sightjack/.siren/inbox",
		"/repo/paintress/.expedition/inbox",
	}

	got := policy.SelectDeliveryInboxes(
		domain.KindImplFeedback,
		inboxes,
		nil,
		domain.CorrectionMetadata{
			SchemaVersion: domain.ImprovementSchemaVersion,
			Outcome:       domain.ImprovementOutcomeEscalated,
			RetryAllowed:  domain.BoolPtr(false),
			RoutingMode:   domain.RoutingModeEscalate,
			TargetAgent:   "paintress",
		},
	)

	if len(got) != 1 || got[0] != inboxes[1] {
		t.Fatalf("SelectDeliveryInboxes() = %v, want [%q]", got, inboxes[1])
	}
}

func TestSelectDeliveryInboxes_FallsBackToRouteFanout(t *testing.T) {
	inboxes := []string{
		"/repo/sightjack/.siren/inbox",
		"/repo/paintress/.expedition/inbox",
	}

	got := policy.SelectDeliveryInboxes(domain.KindDesignFeedback, inboxes, nil, domain.CorrectionMetadata{})
	if len(got) != len(inboxes) {
		t.Fatalf("len(SelectDeliveryInboxes()) = %d, want %d", len(got), len(inboxes))
	}
	for i := range inboxes {
		if got[i] != inboxes[i] {
			t.Fatalf("SelectDeliveryInboxes()[%d] = %q, want %q", i, got[i], inboxes[i])
		}
	}
}
