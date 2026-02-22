# 0002. stdio Convention (stdout=data, stderr=logs)

**Date:** 2026-02-23
**Status:** Accepted

## Context

CLI tools in the phonewave ecosystem produce both structured output (data) and
human-readable messages (logs, progress, errors). Mixing these on the same
stream breaks pipeline composability (`tool1 | tool2`) and makes output parsing
unreliable. A cross-tool audit (MY-339) confirmed this separation was already
in practice but not formally documented.

## Decision

Enforce the following stdio convention across all four tools:

1. **stdout**: Machine-readable data only (JSON, YAML, file content).
2. **stderr**: Human-readable messages (logs, progress, warnings, errors).
3. **cobra abstraction**: Use `cmd.OutOrStdout()` for data output and
   `cmd.ErrOrStderr()` for log output. Never use `fmt.Println` or `os.Stdout`
   directly in command implementations.
4. **Package-level log functions**: `phonewave.LogInfo`, `phonewave.LogWarn`,
   `phonewave.LogError` write to stderr via the shared logger.

## Consequences

### Positive
- Pipeline composability: `phonewave status --json | jq .endpoints`
- Testable output: cobra's `SetOut`/`SetErr` enable buffer-based assertions
- Consistent user experience across all four tools

### Negative
- Developers must consciously choose the correct stream for each output
- Existing code using direct `fmt.Print` requires migration
