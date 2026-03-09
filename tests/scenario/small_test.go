//go:build scenario

package scenario_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestScenario_L2_Small verifies phonewave routing with multiple D-Mails:
//
//   - Burst delivery: 2 specifications injected simultaneously
//   - Priority ordering: high (1) and low (3) priority D-Mails
//   - Mixed feedback actions: retry + resolve
//   - Second cycle: retry feedback triggers a follow-up specification
//   - Fan-out: feedback delivered to both .siren/inbox and .expedition/inbox
func TestScenario_L2_Small(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "small")
	obs := NewObserver(ws, t)

	// Start phonewave daemon
	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// === Cycle 1: 2 specifications → 2 reports → 2 feedbacks ===

	// 1. Inject 2 specification D-Mails with different priorities (burst)
	spec1 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-high-001",
		"kind":                 "specification",
		"description":          "High priority specification",
		"priority":             "1",
	}, "# High Priority Spec\n\n## Actions\n\n- [add_dod] TEST-001: Critical fix")

	spec2 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-low-002",
		"kind":                 "specification",
		"description":          "Low priority specification",
		"priority":             "3",
	}, "# Low Priority Spec\n\n## Actions\n\n- [add_dod] TEST-002: Minor improvement")

	ws.InjectDMail(t, ".siren", "outbox", "spec-high-001.md", spec1)
	ws.InjectDMail(t, ".siren", "outbox", "spec-low-002.md", spec2)

	// 2. Wait for both to arrive in .expedition/inbox
	ws.WaitForDMailCount(t, ".expedition", "inbox", 2, 30*time.Second)

	// 3. Verify outbox is empty
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// 4. Verify both D-Mails have correct kind
	verifyAllDMailKinds(t, ws, ".expedition", "inbox", "specification")

	// 5. Inject 2 report D-Mails (burst)
	report1 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-high-001",
		"kind":                 "report",
		"description":          "Report for high priority spec",
		"priority":             "1",
	}, "# Report: High Priority\n\nExpedition completed.\n\n## Results\n\n- TEST-001: resolved")

	report2 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-low-002",
		"kind":                 "report",
		"description":          "Report for low priority spec",
		"priority":             "3",
	}, "# Report: Low Priority\n\nExpedition completed.\n\n## Results\n\n- TEST-002: needs retry")

	ws.InjectDMail(t, ".expedition", "outbox", "report-high-001.md", report1)
	ws.InjectDMail(t, ".expedition", "outbox", "report-low-002.md", report2)

	// 6. Wait for both reports in .gate/inbox
	ws.WaitForDMailCount(t, ".gate", "inbox", 2, 30*time.Second)

	// 7. Verify outbox empty
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)

	// 8. Verify report kinds
	verifyAllDMailKinds(t, ws, ".gate", "inbox", "report")

	// 9. Inject 2 feedback D-Mails with different actions (resolve + retry)
	fbResolve := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "feedback-resolve-001",
		"kind":                 "design-feedback",
		"description":          "Feedback: high priority resolved",
		"action":               "resolve",
		"priority":             "1",
	}, "# Feedback: Resolved\n\nHigh priority spec fully resolved. No issues detected.")

	fbRetry := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "feedback-retry-002",
		"kind":                 "design-feedback",
		"description":          "Feedback: low priority needs retry",
		"action":               "retry",
		"priority":             "3",
	}, "# Feedback: Retry Required\n\nLow priority spec needs re-expedition due to incomplete coverage.")

	ws.InjectDMail(t, ".gate", "outbox", "feedback-resolve-001.md", fbResolve)
	ws.InjectDMail(t, ".gate", "outbox", "feedback-retry-002.md", fbRetry)

	// 10. Wait for fan-out: both feedbacks to .siren/inbox AND .expedition/inbox
	ws.WaitForDMailCount(t, ".siren", "inbox", 2, 30*time.Second)
	// .expedition/inbox already has 2 specs from step 2, now +2 feedbacks = 4
	ws.WaitForDMailCount(t, ".expedition", "inbox", 4, 30*time.Second)

	// 11. Verify outbox empty
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// 12. Verify feedback kinds in .siren/inbox
	verifyAllDMailKinds(t, ws, ".siren", "inbox", "design-feedback")

	// === Cycle 2: retry triggers follow-up specification ===

	// 13. Inject follow-up specification (simulating sightjack responding to retry)
	specRetry := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-retry-003",
		"kind":                 "specification",
		"description":          "Follow-up specification for retry",
		"priority":             "2",
	}, "# Follow-up Specification\n\n## Actions\n\n- [add_dod] TEST-002: Retry with expanded scope")
	ws.InjectDMail(t, ".siren", "outbox", "spec-retry-003.md", specRetry)

	// 14. Wait for delivery
	// .expedition/inbox: had 4 files, now +1 = 5
	ws.WaitForDMailCount(t, ".expedition", "inbox", 5, 30*time.Second)

	// 15. Verify outbox empty
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// 16. Inject follow-up report
	reportRetry := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-retry-003",
		"kind":                 "report",
		"description":          "Follow-up report for retry",
		"priority":             "2",
	}, "# Follow-up Report\n\nRetry expedition completed.\n\n## Results\n\n- TEST-002: resolved on retry")
	ws.InjectDMail(t, ".expedition", "outbox", "report-retry-003.md", reportRetry)

	// 17. Wait for report delivery
	// .gate/inbox: had 2, now +1 = 3
	ws.WaitForDMailCount(t, ".gate", "inbox", 3, 30*time.Second)

	// 18. Verify outbox empty
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)

	// 19. Inject follow-up feedback (resolve this time)
	fbResolve2 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "feedback-resolve-003",
		"kind":                 "design-feedback",
		"description":          "Feedback: retry now resolved",
		"action":               "resolve",
		"priority":             "2",
	}, "# Feedback: Resolved on Retry\n\nFollow-up spec fully resolved.")
	ws.InjectDMail(t, ".gate", "outbox", "feedback-resolve-003.md", fbResolve2)

	// 20. Wait for fan-out
	ws.WaitForDMailCount(t, ".siren", "inbox", 3, 30*time.Second)
	// .expedition/inbox: had 5, now +1 = 6
	ws.WaitForDMailCount(t, ".expedition", "inbox", 6, 30*time.Second)

	// 21. Verify outbox empty
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// 22. Final state: all outboxes empty
	obs.AssertAllOutboxEmpty()
}

// verifyAllDMailKinds checks that every .md file in a mailbox has the expected kind.
func verifyAllDMailKinds(t *testing.T, ws *Workspace, toolDir, sub, expectedKind string) {
	t.Helper()
	dir := filepath.Join(ws.RepoPath, toolDir, sub)
	files := ws.ListFiles(t, dir)
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		path := filepath.Join(dir, f)
		fm, _ := ws.ReadDMail(t, path)
		kind, ok := fm["kind"].(string)
		if !ok {
			t.Errorf("%s/%s/%s: missing kind in frontmatter", toolDir, sub, f)
			continue
		}
		if kind != expectedKind {
			t.Errorf("%s/%s/%s: got kind %q, want %q", toolDir, sub, f, kind, expectedKind)
		}
	}
}
