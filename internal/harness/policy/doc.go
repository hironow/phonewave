// Package policy implements deterministic decision functions that require
// no LLM interaction. Routing rules, delivery dedup, and idempotency
// checks live here.
//
// policy may only import domain (+ stdlib). It must not import verifier,
// filter, usecase, session, cmd, platform, or eventsource.
//
// See: AutoHarness (arxiv 2603.03329v1) — "Harness as Policy" spectrum.
package policy
