# Scenario Test Gap Fixes Design

**Date:** 2026-03-02
**Status:** Accepted

## Context

Cross-tool scenario tests (L1-L4) for amadeus, paintress, sightjack were implemented and all pass. During implementation, 3 gaps were discovered that affect the D-Mail closed loop and test infrastructure.

## Gaps

### Gap 1: sightjack feedback route missing (CRITICAL)

sightjack's `ComposeFeedback()` (`internal/session/dmail.go:614`) writes `kind: feedback` D-Mails to `.siren/outbox` after wave apply. However, sightjack's `dmail-sendable/SKILL.md` only declares `produces: [specification, report]`. Without `feedback` in produces, phonewave cannot derive a route from `.siren/outbox` for feedback D-Mails.

**Evidence:** `ERR Deliver .siren/outbox/feedback-auth-auth-w1.md: no route for kind="feedback" from .siren/outbox`

**Impact:** O2 feedback loop (sightjack -> amadeus) is broken.

### Gap 2: phonewave pt_expedition.txt fixture incomplete (HIGH)

phonewave's `tests/scenario/testdata/fixtures/*/pt_expedition.txt` lacks `__EXPEDITION_REPORT__` / `__EXPEDITION_END__` markers required by paintress's `ParseReport`. The paintress-side fixture was already fixed during paintress scenario test implementation, but phonewave's copy was not updated.

**Impact:** phonewave closed-loop scenario tests may fail when paintress parses the fake expedition report.

### Gap 3: sightjack --auto-approve scope limited (HIGH)

`--auto-approve` only covers the convergence gate (`BuildApprover` in `gate.go:124`). The interactive wave selection/approval loop (`runInteractiveLoop` in `session.go`) reads from stdin via `bufio.Scanner` directly, bypassing the Approver interface.

**Impact:** sightjack cannot run fully non-interactively, preventing automated D-Mail-driven workflows.

## Design

### Fix 1: Add feedback to sightjack SKILL.md produces

Modify `sightjack/templates/skills/dmail-sendable/SKILL.md` to add `kind: feedback` to the produces list. This enables phonewave's `DeriveRoutes()` to automatically create a feedback route from `.siren/outbox`.

### Fix 2: Update phonewave pt_expedition.txt fixtures

Copy the fixed fixture format (with `__EXPEDITION_REPORT__` / `__EXPEDITION_END__` markers) from paintress to phonewave's scenario test fixtures. Also update the unified fake-claude's `defaultPaintressResponse`.

### Fix 3: Extend --auto-approve to cover wave selection/approval

When `cfg.Gate.AutoApprove` is true, `runInteractiveLoop()` will:
1. Auto-select the first available wave (skip `PromptWaveSelection`)
2. Auto-approve all actions (skip `PromptWaveApproval`)
3. Continue processing next-gen waves until exhausted, then exit

No new flags. `--auto-approve` semantics expand from "convergence gate only" to "all interactive prompts". This is not a breaking change since the flag previously required manual stdin input to be useful.

## Test Strategy

- Fix 1: Re-run `sightjack just test-scenario-min` — verify no ERR route log
- Fix 2: Re-run `phonewave just test-scenario-min` — verify paintress step succeeds
- Fix 3: Remove stdin hack from sightjack scenario harness, verify tests pass with `--auto-approve` alone
