//go:build scenario

package scenario_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestScenario_CorrectiveMetadataRoundtrip verifies that phonewave preserves
// all corrective improvement metadata fields during D-Mail routing AND
// respects target_agent narrowing.
//
// Flow:
//  1. Inject implementation-feedback D-Mail with target_agent=sightjack into .gate/outbox
//  2. phonewave narrows delivery to .siren/inbox only (target_agent takes precedence)
//  3. Assert all 7 metadata fields are preserved in the delivered copy
//  4. Assert .expedition/inbox does NOT receive the D-Mail (narrowed out)
//
// This tests both metadata preservation and the targets > target_agent > fallback
// routing contract documented in SelectDeliveryInboxes.
func TestScenario_CorrectiveMetadataRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Inject corrective D-Mail with full improvement metadata.
	// target_agent=sightjack narrows delivery to .siren/inbox only.
	correctiveDMail := FormatDMailWithMetadata(
		map[string]string{
			"dmail-schema-version": "1",
			"name":                 "corrective-roundtrip-001",
			"kind":                 "implementation-feedback",
			"description":          "Corrective metadata roundtrip test",
			"severity":             "high",
			"action":               "escalate",
		},
		map[string]string{
			"routing_mode":    "escalate",
			"target_agent":    "sightjack",
			"provider_state":  "active",
			"correlation_id":  "corr-roundtrip-001",
			"trace_id":        "trace-roundtrip-001",
			"owner_history":   "amadeus,sightjack",
			"routing_history": "amadeus:retry,sightjack:escalate",
		},
		"# Corrective Metadata Roundtrip\n\nVerifying metadata preservation through phonewave routing.",
	)
	ws.InjectDMail(t, ".gate", "outbox", "corrective-roundtrip-001.md", correctiveDMail)

	// Wait for delivery to .siren/inbox (target_agent=sightjack → narrowed to siren only)
	sirenPath := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)

	// Source should be cleaned up
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// Assert kind preserved
	obs.AssertDMailKind(sirenPath, "implementation-feedback")

	// Assert all 7 metadata fields preserved
	metadataChecks := map[string]string{
		"routing_mode":    "escalate",
		"target_agent":    "sightjack",
		"provider_state":  "active",
		"correlation_id":  "corr-roundtrip-001",
		"trace_id":        "trace-roundtrip-001",
		"owner_history":   "amadeus,sightjack",
		"routing_history": "amadeus:retry,sightjack:escalate",
	}
	for key, want := range metadataChecks {
		obs.AssertDMailMetadata(sirenPath, key, want)
	}

	// Verify source identity
	sirenFm, _ := ws.ReadDMail(t, sirenPath)
	sirenName, _ := sirenFm["name"].(string)
	if sirenName != "corrective-roundtrip-001" {
		t.Errorf("siren name = %q, want %q", sirenName, "corrective-roundtrip-001")
	}
	if filepath.Base(sirenPath) != "corrective-roundtrip-001.md" {
		t.Errorf("siren file = %q, want corrective-roundtrip-001.md", filepath.Base(sirenPath))
	}

	// Assert .expedition/inbox did NOT receive the D-Mail.
	// target_agent=sightjack narrows delivery to siren only.
	obs.AssertMailboxState(map[string]int{
		".expedition/inbox": 0,
	})
}
