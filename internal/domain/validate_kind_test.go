package domain_test

import (
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestValidateKind_AcceptsStallEscalation(t *testing.T) {
	// given — stall-escalation is an official D-Mail kind per SPEC-001
	kind := "stall-escalation"

	// when
	err := domain.ValidateKind(kind)

	// then
	if err != nil {
		t.Errorf("ValidateKind(%q) = %v, want nil", kind, err)
	}
}

func TestValidateKind_RejectsUnknownKind(t *testing.T) {
	// given
	kind := "foo-bar"

	// when
	err := domain.ValidateKind(kind)

	// then
	if err == nil {
		t.Errorf("ValidateKind(%q) = nil, want error", kind)
	}
}

func TestValidateKind_AcceptsAllOfficialKinds(t *testing.T) {
	officialKinds := []string{
		"specification",
		"report",
		"design-feedback",
		"implementation-feedback",
		"convergence",
		"ci-result",
		"stall-escalation",
	}

	for _, kind := range officialKinds {
		t.Run(kind, func(t *testing.T) {
			if err := domain.ValidateKind(kind); err != nil {
				t.Errorf("ValidateKind(%q) = %v, want nil", kind, err)
			}
		})
	}
}
