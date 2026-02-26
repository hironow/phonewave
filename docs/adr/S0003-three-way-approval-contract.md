# S0003. Three-Way Approval Contract

**Date:** 2026-02-25
**Status:** Accepted

## Context

sightjack (ADR 0006) and paintress (ADR 0010) independently designed approval
gate systems for human-in-the-loop decisions. Both tools converged on an
identical three-way return contract and the same set of three Approver
implementations, despite being designed without cross-tool coordination.

The convergence covers:

- Return contract: `(bool, error)` with three distinct semantic outcomes
- Fail-closed default: errors always deny
- Three implementations: StdinApprover, CmdApprover, AutoApprover
- Notifier as a separate, non-blocking interface

## Decision

Recognize the three-way approval contract as the shared pattern for
human-in-the-loop gates in the phonewave ecosystem.

### Three-Way Return Contract

`Approver.RequestApproval(ctx, message)` returns `(approved bool, err error)`:

| Return | Meaning | Behavior |
|--------|---------|----------|
| `(true, nil)` | Approved | Proceed normally |
| `(false, nil)` | Denied | Clean exit (exit code 0) |
| `(false, err)` | Infrastructure failure | Fail-closed abort (exit code 1) |

The critical invariant: **`(true, err)` is never returned**. Errors always
result in denial, ensuring fail-closed semantics.

### Three Implementations

1. **StdinApprover**: Interactive terminal prompt. Reads from stdin (or
   `/dev/tty` for pipe compatibility). `y`/`yes` = approve, anything else =
   deny. Uses goroutine + channel for context cancellation.

2. **CmdApprover**: External command execution via `sh -c`. Exit code 0 =
   approve. `*exec.ExitError` (non-zero exit) = deny. Other errors (binary
   not found, permission denied) = infrastructure failure. Supports
   `{message}` placeholder with shell quoting.

3. **AutoApprover**: Always returns `(true, nil)`. Used with `--auto-approve`
   for CI/CD environments.

### Notifier Interface

`Notifier.Notify(ctx, message)` is fire-and-forget, non-blocking (30s
timeout), and independent of Approver. Notification failure does not block
or affect approval decisions.

### Tool-Specific Variants

| Aspect | sightjack | paintress |
|--------|-----------|-----------|
| **Gate trigger** | Convergence D-Mail in inbox | HIGH severity D-Mail in inbox |
| **Gate timing** | Per-wave with redrain loop | Session-level (single pre-flight check) |
| **Redrain** | Re-drains inbox after approval to catch racing D-Mails | Not implemented (mid-expedition arrivals only notify) |
| **CLI flags** | `--notify-cmd`, `--approve-cmd`, `--auto-approve` on `run` | Same flags on `run` |
| **ADRs** | 0006 (convergence gate design) | 0010 (three-way approval contract) |

## Consequences

### Positive

- Shared vocabulary for approval gates across tools
- New tools can adopt the pattern by implementing the three Approver variants
- Fail-closed semantics are documented as a cross-tool invariant

### Negative

- Tools must still decide between gate timing strategies (per-event redrain vs
  session-level) based on their specific requirements

### Neutral

- phonewave and amadeus do not currently implement approval gates; this ADR
  does not mandate adoption
