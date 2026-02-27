# 0008. Signal Context Propagation and Daemon Lifecycle

**Date:** 2026-02-23
**Status:** Superseded by S0006

## Context

phonewave is unique among the four tools as a long-running daemon process.
The `phonewave run` command starts a file watcher that blocks until terminated.
Correct signal handling is critical: SIGINT/SIGTERM must propagate from the OS
through cobra to the daemon's fsnotify event loop for graceful shutdown (PID
file cleanup, delivery log flush, watcher close).

Early implementations used ad-hoc signal handlers inside the daemon, leading to
race conditions between shutdown and in-flight deliveries.

## Decision

Implement a signal propagation chain from `main.go` through cobra to the daemon:

1. **`signal.NotifyContext` in `main.go`**: Create a context that cancels on
   SIGINT or SIGTERM. This is the single point of signal registration for the
   entire process.
2. **`ExecuteContext(ctx)`**: Pass the signal-aware context to cobra's root
   command. All subcommands receive it via `cmd.Context()`.
3. **Daemon uses `cmd.Context()`**: The `phonewave run` handler passes
   `cmd.Context()` to `Daemon.Run(ctx)`. Context cancellation triggers the
   daemon's shutdown sequence.
4. **`SilenceUsage` and `SilenceErrors`**: Both are set on the root command
   to prevent cobra from printing usage/errors on signal-induced cancellation.
5. **`ErrUpdateAvailable` sentinel**: The `update --check` command returns a
   sentinel error to signal "update available" (exit code 1) without printing
   an error message. `main.go` checks for this sentinel before writing to
   stderr.

## Consequences

### Positive

- Single signal registration point eliminates race conditions
- Context cancellation flows naturally through Go's context hierarchy
- Daemon shutdown is testable by cancelling the context in unit tests
- No signal handlers inside library code — only in `main.go`

### Negative

- cobra's `PersistentPostRunE` is skipped when `RunE` returns an error,
  so cleanup (e.g., tracer shutdown) must use `defer` in `main.go`
- Sentinel error pattern (`ErrUpdateAvailable`) requires callers to
  distinguish "expected" errors from real failures
