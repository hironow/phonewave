// Package verifier implements validation rules that require no LLM
// interaction. D-Mail schema validation, payload integrity checks,
// and provider error classification live here.
//
// verifier may import domain and harness/policy (+ stdlib).
// It must not import filter, usecase, session, cmd, platform, or eventsource.
//
// See: AutoHarness (arxiv 2603.03329v1) — "Harness as Policy" spectrum.
package verifier
