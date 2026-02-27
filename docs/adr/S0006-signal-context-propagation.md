# S0006. Signal Context Propagation and Process Lifecycle

**Date:** 2026-02-27
**Status:** Accepted
**Supersedes:** phonewave ADR 0008 (tool-specific)

## Context

All four tools (phonewave, sightjack, paintress, amadeus) need correct signal
handling for graceful shutdown. phonewave is a long-running daemon; sightjack
and paintress run interactive Claude sessions; amadeus performs batch checks.
In all cases, SIGINT/SIGTERM must propagate from the OS through cobra to the
active operation for clean resource release.

Ad-hoc signal handlers inside session code lead to race conditions and
inconsistent shutdown behavior across tools.

## Decision

Implement a uniform signal propagation chain across all four tools:

1. **`signal.NotifyContext` in `main.go`**: Create a context that cancels on
   SIGINT or SIGTERM. This is the single point of signal registration for the
   entire process.
2. **`ExecuteContext(ctx)`**: Pass the signal-aware context to cobra's root
   command. All subcommands receive it via `cmd.Context()`.
3. **Session code uses `cmd.Context()`**: Long-running handlers pass
   `cmd.Context()` to their orchestrators. Context cancellation triggers the
   shutdown sequence.
4. **`SilenceUsage` and `SilenceErrors`**: Both set on root command to prevent
   cobra from printing usage/errors on signal-induced cancellation.
5. **Sentinel errors** (e.g., `ErrUpdateAvailable`): `main.go` checks for
   sentinels before writing to stderr.
6. **No signal handlers in library code**: Only `main.go` registers signals.

## Consequences

### Positive

- Single signal registration point eliminates race conditions
- Context cancellation flows naturally through Go's context hierarchy
- Shutdown is testable by cancelling the context in unit tests
- Consistent behavior across all four tools

### Negative

- cobra's `PersistentPostRunE` is skipped when `RunE` returns an error,
  so cleanup (e.g., tracer shutdown) must use `defer` in `main.go` or
  `cobra.OnFinalize`
- Sentinel error pattern requires callers to distinguish "expected" errors
  from real failures
