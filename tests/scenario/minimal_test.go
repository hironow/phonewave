//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
)

// TestScenario_L1_Minimal verifies phonewave routing for one complete closed loop:
//
//  1. specification: .siren/outbox → .expedition/inbox
//  2. report:        .expedition/outbox → .gate/inbox
//  3. feedback:      .gate/outbox → .siren/inbox AND .expedition/inbox (fan-out)
//
// This test injects D-Mails directly into outbox directories (bypassing tool execution)
// to isolate phonewave's routing and delivery logic. Tool integration is tested in L2+.
func TestScenario_L1_Minimal(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Setup workspace with L1 fixtures
	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	// 1. Start phonewave daemon
	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw) // dump on failure for debugging

	// --- Route 1: specification (.siren/outbox → .expedition/inbox) ---

	// 2. Inject specification D-Mail into .siren/outbox
	specDMail := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-minimal-001",
		"kind":                 "specification",
		"description":          "Minimal test specification",
	}, "# Minimal Specification\n\n## Actions\n\n- [add_dod] TEST-001: Test action item")
	ws.InjectDMail(t, ".siren", "outbox", "spec-minimal-001.md", specDMail)

	// 3. Wait for delivery to .expedition/inbox
	specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)

	// 4. Verify source removed from outbox
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// 5. Verify delivered D-Mail kind
	obs.AssertDMailKind(specPath, "specification")

	// --- Route 2: report (.expedition/outbox → .gate/inbox) ---

	// 6. Inject report D-Mail into .expedition/outbox
	reportDMail := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-minimal-001",
		"kind":                 "report",
		"description":          "Minimal test report",
	}, "# Minimal Report\n\nExpedition completed successfully.\n\n## Results\n\n- TEST-001: resolved")
	ws.InjectDMail(t, ".expedition", "outbox", "report-minimal-001.md", reportDMail)

	// 7. Wait for delivery to .gate/inbox
	reportPath := ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)

	// 8. Verify source removed from outbox
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)

	// 9. Verify delivered D-Mail kind
	obs.AssertDMailKind(reportPath, "report")

	// --- Route 3: feedback (.gate/outbox → .siren/inbox + .expedition/inbox) ---

	// 10. Inject feedback D-Mail into .gate/outbox
	feedbackDMail := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "feedback-minimal-001",
		"kind":                 "design-feedback",
		"description":          "Minimal test feedback",
		"action":               "resolve",
	}, "# Minimal Feedback\n\nAll checks passed. No issues detected.")
	ws.InjectDMail(t, ".gate", "outbox", "feedback-minimal-001.md", feedbackDMail)

	// 11. Wait for fan-out delivery
	//     .siren/inbox: first file (empty before)
	feedbackSiren := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	//     .expedition/inbox: already has spec from step 3, wait for 2nd file
	ws.WaitForDMailCount(t, ".expedition", "inbox", 2, 30*time.Second)

	// 12. Verify source removed from outbox
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// 13. Verify delivered feedback kind
	obs.AssertDMailKind(feedbackSiren, "design-feedback")

	// 14. Wait for closed loop completion (spec→report→feedback all delivered)
	obs.WaitForClosedLoop(60 * time.Second)

	// 15. Final state: all outboxes empty
	obs.AssertAllOutboxEmpty()
}
