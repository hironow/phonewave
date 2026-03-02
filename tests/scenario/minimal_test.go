//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
)

// TestScenario_L1_Minimal verifies one complete closed loop:
//
//  1. sightjack produces specification -> .siren/outbox
//  2. phonewave routes specification to .expedition/inbox
//  3. paintress processes specification, produces report -> .expedition/outbox
//  4. phonewave routes report to .gate/inbox
//  5. amadeus processes report, produces feedback -> .gate/outbox
//  6. phonewave routes feedback to .siren/inbox AND .expedition/inbox
//
// NOTE: sightjack run is interactive (wave selection requires stdin).
// The current RunSightjack helper uses CombinedOutput which provides no stdin,
// so the interactive loop will receive EOF and quit before approving any waves.
// This test will need iteration on the harness (e.g., RunSightjackInteractive
// with stdin pipe providing "1\na\n" for wave selection + approval) or an
// alternative approach (e.g., sightjack scan + manual D-Mail injection).
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

	// 2. Run sightjack -> produces specification
	err := ws.RunSightjack(t, ctx, "run", "--auto-approve", "--approve-cmd", "exit 0")
	if err != nil {
		t.Fatalf("sightjack run failed: %v", err)
	}

	// 3. Wait for specification delivery to .expedition/inbox
	ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)

	// 4. Run paintress -> consumes spec, produces report
	err = ws.RunPaintress(t, ctx, "run", "--auto-approve", "--no-dev", "--workers", "0")
	if err != nil {
		t.Fatalf("paintress run failed: %v", err)
	}

	// 5. Wait for report delivery to .gate/inbox
	ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)

	// 6. Run amadeus -> consumes report, produces feedback
	err = ws.RunAmadeus(t, ctx, "check", "--auto-approve", "--approve-cmd", "exit 0")
	if err != nil {
		t.Fatalf("amadeus check failed: %v", err)
	}

	// 7. Wait for feedback delivery
	ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)

	// 8. Verify final state
	obs.AssertAllOutboxEmpty()
	obs.AssertArchiveContains(".siren", []string{"specification"})
	obs.AssertArchiveContains(".expedition", []string{"report"})
	obs.AssertArchiveContains(".gate", []string{"feedback"})
}
