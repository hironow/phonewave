package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hironow/phonewave/internal/domain"
)

func TestStatusReport_FormatText_IncludesProviderState(t *testing.T) {
	report := domain.StatusReport{
		ProviderState:       string(domain.ProviderStateWaiting),
		ProviderReason:      "delivery_retry_backoff",
		ProviderRetryBudget: 1,
		ProviderResumeAt:    time.Date(2026, 4, 5, 16, 30, 0, 0, time.UTC),
		ProviderResumeWhen:  "backoff-elapses",
	}

	got := report.FormatText()

	for _, want := range []string{
		"Provider:",
		"waiting",
		"delivery_retry_backoff",
		"Retry budget:",
		"Resume when:",
		"backoff-elapses",
		"Resume at:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatText() missing %q in %q", want, got)
		}
	}
}
