# stdio Convention

Phonewave follows the Unix convention of separating machine-readable data from human-readable diagnostics across standard streams.

## Stream Assignment

| Stream | Purpose | Implementation |
|--------|---------|----------------|
| **stdout** | Machine-readable output (JSON, delivery results) | `cmd.OutOrStdout()` |
| **stderr** | Human-readable progress, logs, errors | `cmd.ErrOrStderr()` |
| **stdin** | Not used (daemon mode) | — |

## Cobra Wiring

All cobra subcommands MUST use cobra's stream accessors:

```go
logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)
```

Rules:

- Use `cmd.OutOrStdout()` for data output — never `os.Stdout` directly
- Use `cmd.ErrOrStderr()` for logs — never `os.Stderr` directly
- This enables cobra's `cmd.SetOut()` / `cmd.SetErr()` for testing

### Exceptions

Direct `os.Stderr` is acceptable only where cobra's `cmd` is unavailable:

| Location | Reason |
|----------|--------|
| `cmd/phonewave/main.go` | Error handling after `root.ExecuteContext()` returns |
| `internal/tools/docgen/main.go` | Standalone tool outside cobra |

## Pipeline Compatibility

The stream separation ensures correct behavior in Unix pipelines:

```bash
phonewave status --json | jq '.watchers'    # stdout = JSON only
phonewave status --json 2>/dev/null         # suppress stderr logs
phonewave status --json 2>delivery.log      # split logs to file
```
