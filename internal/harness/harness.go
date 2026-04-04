// Package harness mediates between the task environment and decision logic.
// It is the single import surface for all decision and validation logic.
// Internal sub-packages (policy, verifier) represent the LLM-dependence
// spectrum but are not imported directly by callers.
//
// phonewave has no filter/ sub-package because it does not call LLMs.
//
// See: AutoHarness (arxiv 2603.03329v1) — "Harness as Policy" spectrum.
package harness

// --- policy layer (deterministic decisions, no LLM) ---
// Currently empty: routing and orphan detection are tightly coupled to
// domain aggregate types and remain in internal/domain/router.go.

// --- verifier layer (validation rules, no LLM) ---
// Currently empty: D-Mail validation (ExtractDMailKind, ValidateKind)
// is called internally within domain/ and must stay there.
// IsDMailFile is a thin filter also kept in domain/ for now.
