// Package harness mediates between the task environment and decision logic.
// It is the single import surface for all decision and validation logic.
//
// phonewave has no active harness code because routing and validation
// logic is tightly coupled to domain aggregate types in internal/domain/.
// Sub-packages (policy, verifier, filter) will be created when decision
// logic is extracted from domain.
//
// Semgrep rules (.semgrep/layers-harness.yaml) enforce the import
// boundaries for this layer across all 4 tools.
//
// See: AutoHarness (arxiv 2603.03329v1) -- "Harness as Policy" spectrum.
package harness
