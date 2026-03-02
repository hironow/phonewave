//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
)

// TestScenario_L3_Middle verifies phonewave routing under concurrent load:
//
//   - Burst of 3 specifications injected simultaneously
//   - Convergence D-Mail routing (kind: convergence → .siren/inbox)
//   - Interleaved inject: new D-Mails added while previous routing in flight
//   - Multiple cycles with growing inbox counts
//   - Fan-out consistency across all delivery points
func TestScenario_L3_Middle(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "middle")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// === Phase 1: Burst of 3 specifications ===

	for i, spec := range []struct {
		name     string
		priority string
		body     string
	}{
		{"spec-auth-001", "1", "# Auth Spec\n\n## Actions\n\n- [add_dod] AUTH-001: OAuth2 flow"},
		{"spec-perf-002", "2", "# Perf Spec\n\n## Actions\n\n- [add_dod] PERF-001: Query optimization"},
		{"spec-docs-003", "3", "# Docs Spec\n\n## Actions\n\n- [add_dod] DOCS-001: API documentation"},
	} {
		_ = i
		dmail := FormatDMail(map[string]string{
			"dmail-schema-version": "1",
			"name":                 spec.name,
			"kind":                 "specification",
			"description":          "Spec: " + spec.name,
			"priority":             spec.priority,
		}, spec.body)
		ws.InjectDMail(t, ".siren", "outbox", spec.name+".md", dmail)
	}

	// Wait for all 3 specs in .expedition/inbox
	ws.WaitForDMailCount(t, ".expedition", "inbox", 3, 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// === Phase 2: Convergence D-Mail ===

	convergenceDMail := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "convergence-auth-001",
		"kind":                 "convergence",
		"description":          "Recurring drift in auth module",
		"severity":             "medium",
	}, "# Convergence: Auth Module\n\nRecurring issues detected in authentication module across 3 cycles.")

	// Convergence route CONTRACTED: .gate/outbox → .siren/inbox
	// (amadeus produces convergence → sightjack consumes convergence)
	ws.InjectDMail(t, ".gate", "outbox", "convergence-auth-001.md", convergenceDMail)

	// Wait for convergence delivery to .siren/inbox
	convPath := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	obs.AssertDMailKind(convPath, "convergence")

	// === Phase 3: Reports (interleaved with phase 2 processing) ===

	for _, report := range []struct {
		name     string
		priority string
		body     string
	}{
		{"report-auth-001", "1", "# Auth Report\n\n## Results\n\n- AUTH-001: implemented"},
		{"report-perf-002", "2", "# Perf Report\n\n## Results\n\n- PERF-001: optimized"},
		{"report-docs-003", "3", "# Docs Report\n\n## Results\n\n- DOCS-001: documented"},
	} {
		dmail := FormatDMail(map[string]string{
			"dmail-schema-version": "1",
			"name":                 report.name,
			"kind":                 "report",
			"description":          "Report: " + report.name,
			"priority":             report.priority,
		}, report.body)
		ws.InjectDMail(t, ".expedition", "outbox", report.name+".md", dmail)
	}

	// Wait for all 3 reports in .gate/inbox
	// Convergence is NOT routed here — only to .siren/inbox (contract: .gate/outbox → .siren/inbox)
	ws.WaitForDMailCount(t, ".gate", "inbox", 3, 30*time.Second)
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)

	// Verify convergence did NOT leak to .expedition/inbox (paintress does not consume convergence)
	expeditionCount := countMDFiles(ws.ToolDir(".expedition") + "/inbox")
	if expeditionCount != 3 {
		t.Errorf("convergence leak check: .expedition/inbox has %d files (expected 3 specs only)", expeditionCount)
	}

	// === Phase 4: 3 feedbacks (all resolve) ===

	for _, fb := range []struct {
		name     string
		priority string
		action   string
		body     string
	}{
		{"feedback-auth-001", "1", "resolve", "# Feedback: Auth\n\nAuth implementation verified."},
		{"feedback-perf-002", "2", "resolve", "# Feedback: Perf\n\nPerformance meets targets."},
		{"feedback-docs-003", "3", "resolve", "# Feedback: Docs\n\nDocumentation complete."},
	} {
		dmail := FormatDMail(map[string]string{
			"dmail-schema-version": "1",
			"name":                 fb.name,
			"kind":                 "feedback",
			"description":          "Feedback: " + fb.name,
			"priority":             fb.priority,
			"action":               fb.action,
		}, fb.body)
		ws.InjectDMail(t, ".gate", "outbox", fb.name+".md", dmail)
	}

	// Wait for fan-out: 3 feedbacks to .siren/inbox (+ 1 convergence already there = 4 total)
	ws.WaitForDMailCount(t, ".siren", "inbox", 4, 30*time.Second)
	// .expedition/inbox: 3 specs + 3 feedbacks = 6
	ws.WaitForDMailCount(t, ".expedition", "inbox", 6, 30*time.Second)
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// === Final verification ===

	obs.AssertAllOutboxEmpty()
}
