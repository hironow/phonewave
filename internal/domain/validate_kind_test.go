package domain_test

import (
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestParseDMailKind_AcceptsStallEscalation(t *testing.T) {
	// given — stall-escalation is an official D-Mail kind per SPEC-001
	kind := "stall-escalation"

	// when
	_, err := domain.ParseDMailKind(kind)

	// then
	if err != nil {
		t.Errorf("ParseDMailKind(%q) = %v, want nil", kind, err)
	}
}

func TestParseDMailKind_RejectsUnknownKind(t *testing.T) {
	// given
	kind := "foo-bar"

	// when
	_, err := domain.ParseDMailKind(kind)

	// then
	if err == nil {
		t.Errorf("ParseDMailKind(%q) = nil, want error", kind)
	}
}

func TestParseDMailKind_AcceptsAllOfficialKinds(t *testing.T) {
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
			if _, err := domain.ParseDMailKind(kind); err != nil {
				t.Errorf("ParseDMailKind(%q) = %v, want nil", kind, err)
			}
		})
	}
}
