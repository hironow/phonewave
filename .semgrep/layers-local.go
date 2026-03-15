package testdata

// ==========================================================================
// layers-local.yaml test fixture
// Covers phonewave-specific rules:
//   - daemon-session-direct-access
//   - no-state-dir-literal-in-path-join
// ==========================================================================

// --- Rule: daemon-session-direct-access ---

func badDaemonSessionAccessLocal(d struct {
	session struct{ HasErrorQueue func() bool }
}) {
	// ruleid: daemon-session-direct-access
	d.session.HasErrorQueue()
}

func goodDaemonForwardingLocal(d struct{ hasErrorQueue func() bool }) {
	// ok: daemon-session-direct-access
	d.hasErrorQueue()
}
