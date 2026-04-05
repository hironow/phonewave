// Package harness mediates between the task environment and deterministic
// decision logic. It is the single import surface for extracted policy helpers.
//
// phonewave remains policy-first / harness-thin: most routing/runtime logic
// still lives in internal/domain, internal/session, and internal/usecase.
// This package exposes extracted helpers as those seams become stable.
//
// Semgrep rules (.semgrep/layers-harness.yaml) enforce the import
// boundaries for this layer across all 4 tools.
//
// See: AutoHarness (arxiv 2603.03329v1) -- "Harness as Policy" spectrum.
package harness

import "github.com/hironow/phonewave/internal/harness/policy"

// SelectDeliveryInboxes applies delivery precedence without side effects.
var SelectDeliveryInboxes = policy.SelectDeliveryInboxes
