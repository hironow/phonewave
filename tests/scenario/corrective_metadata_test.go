//go:build scenario

package scenario_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestScenario_CorrectiveMetadataRoundtrip verifies that phonewave preserves
// all corrective improvement metadata fields during D-Mail routing.
//
// Flow:
//  1. Inject implementation-feedback D-Mail with 7 metadata fields into .gate/outbox
//  2. phonewave routes to .siren/inbox and .expedition/inbox (fan-out)
//  3. Assert all metadata fields are preserved in delivered copies
//
// This tests the correctness path for corrective metadata propagation
// from amadeus → phonewave → sightjack/paintress.
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

	// Inject corrective D-Mail with full improvement metadata
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
			"routing_mode":   "escalate",
			"target_agent":   "sightjack",
			"provider_state": "active",
			"correlation_id": "corr-roundtrip-001",
			"trace_id":       "trace-roundtrip-001",
			"owner_history":  "amadeus,sightjack",
			"routing_history": "amadeus:retry,sightjack:escalate",
		},
		"# Corrective Metadata Roundtrip\n\nVerifying metadata preservation through phonewave routing.",
	)
	ws.InjectDMail(t, ".gate", "outbox", "corrective-roundtrip-001.md", correctiveDMail)

	// Wait for delivery to both inboxes (fan-out: implementation-feedback → siren + expedition)
	sirenPath := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	expeditionPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)

	// Source should be cleaned up
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// Assert kind preserved
	obs.AssertDMailKind(sirenPath, "implementation-feedback")
	obs.AssertDMailKind(expeditionPath, "implementation-feedback")

	// Assert all 7 metadata fields preserved in siren delivery
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

	// Assert all 7 metadata fields preserved in expedition delivery
	for key, want := range metadataChecks {
		obs.AssertDMailMetadata(expeditionPath, key, want)
	}

	// Verify both deliveries reference the same source
	sirenFm, _ := ws.ReadDMail(t, sirenPath)
	expeditionFm, _ := ws.ReadDMail(t, expeditionPath)
	sirenName, _ := sirenFm["name"].(string)
	expeditionName, _ := expeditionFm["name"].(string)
	if sirenName != "corrective-roundtrip-001" {
		t.Errorf("siren name = %q, want %q", sirenName, "corrective-roundtrip-001")
	}
	if expeditionName != "corrective-roundtrip-001" {
		t.Errorf("expedition name = %q, want %q", expeditionName, "corrective-roundtrip-001")
	}

	// Verify file names match
	if filepath.Base(sirenPath) != "corrective-roundtrip-001.md" {
		t.Errorf("siren file = %q, want corrective-roundtrip-001.md", filepath.Base(sirenPath))
	}
	if filepath.Base(expeditionPath) != "corrective-roundtrip-001.md" {
		t.Errorf("expedition file = %q, want corrective-roundtrip-001.md", filepath.Base(expeditionPath))
	}
}
