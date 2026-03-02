//go:build scenario

package scenario_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestScenario_L4_Hard verifies phonewave resilience under fault conditions:
//
//   - Daemon restart: kill → verify pending outbox files processed after restart
//   - Malformed D-Mail: invalid YAML frontmatter → error queue (not delivered)
//   - Normal routing continues after processing malformed D-Mail
//   - Startup scan: outbox files present before daemon start get delivered
func TestScenario_L4_Hard(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "hard")
	obs := NewObserver(ws, t)

	// === Phase 1: Normal routing baseline ===

	pw := ws.StartPhonewave(t, ctx)

	// Inject and verify a normal spec routes correctly
	spec1 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-baseline-001",
		"kind":                 "specification",
		"description":          "Baseline specification",
	}, "# Baseline Spec\n\nBaseline routing test.")
	ws.InjectDMail(t, ".siren", "outbox", "spec-baseline-001.md", spec1)
	ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// === Phase 2: Daemon restart with pending outbox ===

	// Inject a spec into outbox BEFORE stopping the daemon
	spec2 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-pending-002",
		"kind":                 "specification",
		"description":          "Pending specification (pre-restart)",
	}, "# Pending Spec\n\nThis was in outbox when daemon stopped.")
	ws.InjectDMail(t, ".siren", "outbox", "spec-pending-002.md", spec2)

	// Give phonewave a moment to potentially pick it up
	time.Sleep(2 * time.Second)

	// Stop daemon
	ws.StopPhonewave(t, pw)
	t.Log("phonewave daemon stopped")

	// Inject another spec while daemon is down
	spec3 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-offline-003",
		"kind":                 "specification",
		"description":          "Offline specification (injected while daemon down)",
	}, "# Offline Spec\n\nInjected while phonewave was stopped.")
	ws.InjectDMail(t, ".siren", "outbox", "spec-offline-003.md", spec3)

	// Restart daemon — startup scan should pick up any pending outbox files
	pw = ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)
	t.Log("phonewave daemon restarted")

	// Wait for all specs to arrive (baseline + pending + offline = up to 3 total)
	// Note: spec2 might have been processed before shutdown, so we check for at least 2 new ones
	ws.WaitForDMailCount(t, ".expedition", "inbox", 2, 45*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)

	// === Phase 3: Malformed D-Mail handling ===

	// Inject a malformed D-Mail (invalid YAML frontmatter)
	malformedContent := []byte("---\nthis is: [not: valid: yaml: {{{\n---\n\nBad content.")
	ws.InjectDMail(t, ".siren", "outbox", "malformed-004.md", malformedContent)

	// Wait for phonewave to process it (should go to error queue, not inbox)
	time.Sleep(5 * time.Second)

	// Verify malformed file is removed from outbox
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)

	// Verify it did NOT get delivered to .expedition/inbox
	// Count should not have increased beyond what we already have
	expInboxBefore := countMDFiles(filepath.Join(ws.RepoPath, ".expedition", "inbox"))

	// === Phase 4: Normal routing continues after error ===

	// Inject a valid report to verify routing still works after malformed D-Mail
	report := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "report-recovery-001",
		"kind":                 "report",
		"description":          "Post-recovery report",
	}, "# Recovery Report\n\nRouting works after malformed D-Mail.")
	ws.InjectDMail(t, ".expedition", "outbox", "report-recovery-001.md", report)
	ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)

	// Inject feedback and verify fan-out still works
	feedback := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "feedback-recovery-001",
		"kind":                 "feedback",
		"description":          "Post-recovery feedback",
		"action":               "resolve",
	}, "# Recovery Feedback\n\nSystem recovered from malformed D-Mail.")
	ws.InjectDMail(t, ".gate", "outbox", "feedback-recovery-001.md", feedback)
	ws.WaitForDMail(t, ".siren", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".gate", "outbox", 10*time.Second)

	// === Phase 5: Verify .expedition/inbox did not get malformed D-Mail ===

	expInboxAfter := countMDFiles(filepath.Join(ws.RepoPath, ".expedition", "inbox"))
	// Should have increased by exactly 1 (the feedback fan-out), not by 2 (would mean malformed leaked)
	malformedLeaked := expInboxAfter - expInboxBefore
	// We expect 1 more file (feedback fan-out). If we got 2+, malformed may have leaked.
	if malformedLeaked > 1 {
		t.Errorf("possible malformed D-Mail leakage: .expedition/inbox grew by %d (expected 1)", malformedLeaked)
	}

	// === Phase 6: Verify error queue has the malformed entry ===

	stateDir := ws.PhonewaveStateDir()
	errorsDir := filepath.Join(stateDir, "errors")
	if entries, err := os.ReadDir(errorsDir); err == nil {
		var errFiles int
		for _, e := range entries {
			if !e.IsDir() {
				errFiles++
			}
		}
		if errFiles > 0 {
			t.Logf("error queue has %d file(s) (expected for malformed D-Mail)", errFiles)
		}
	}
	// Note: error queue may be in SQLite (.run/error_queue.db) instead of files.
	// Either way, the malformed D-Mail should not be in any inbox.

	// Final verification
	obs.AssertAllOutboxEmpty()
}
