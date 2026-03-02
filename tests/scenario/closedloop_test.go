//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestScenario_ClosedLoop_4Tool verifies the complete 4-tool D-Mail closed loop:
// sightjack → phonewave → paintress → phonewave → amadeus → phonewave → siren+expedition
//
// No manual D-Mail injection — all routing is through real tool execution + phonewave delivery.
func TestScenario_ClosedLoop_4Tool(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	// Start phonewave daemon
	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Step 1: sightjack scan → specification
	err := ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Fatalf("sightjack scan failed: %v", err)
	}
	specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)
	obs.AssertDMailKind(specPath, "specification")
	t.Log("step 1: sightjack → specification delivered to .expedition/inbox")

	// Step 2: paintress expedition → report
	err = ws.RunPaintressExpedition(t, ctx)
	if err != nil {
		t.Fatalf("paintress expedition failed: %v", err)
	}
	reportPath := ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)
	obs.AssertDMailKind(reportPath, "report")
	t.Log("step 2: paintress → report delivered to .gate/inbox")

	// Step 3: amadeus check → feedback (fan-out to .siren/inbox + .expedition/inbox)
	err = ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Log("amadeus check returned exit code 2 (drift detected) — expected")
		} else {
			t.Fatalf("amadeus check failed: %v", err)
		}
	}
	feedbackSiren := ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	obs.AssertDMailKind(feedbackSiren, "feedback")

	// Verify feedback fan-out to .expedition/inbox.
	// Paintress consumed earlier D-Mails (spec etc.) in step 2, so only the
	// amadeus feedback remains. Verify it arrived and has kind=feedback.
	feedbackExpedition := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	obs.AssertDMailKind(feedbackExpedition, "feedback")

	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)
	t.Log("step 3: amadeus → feedback delivered to .siren/inbox + .expedition/inbox (fan-out verified)")

	// Final: all outboxes empty, loop complete
	obs.AssertAllOutboxEmpty()
	t.Log("4-tool closed loop complete: spec → report → feedback, all outboxes empty")
}
