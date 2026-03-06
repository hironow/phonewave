# phonewave scenario tests

## Prerequisites

- Go 1.26+ (all 4 repos must use the same toolchain)
- Sibling repos at the same parent directory:
  - `phonewave/`, `sightjack/`, `paintress/`, `amadeus/`
  - Override with env vars: `PHONEWAVE_REPO`, `SIGHTJACK_REPO`, `PAINTRESS_REPO`, `AMADEUS_REPO`

## Running

```bash
# L1 minimal (single closed loop, ~12s)
just test-scenario-min

# L2 small (multi-issue + retry, ~14s)
just test-scenario-small

# L3 middle (parallel + convergence routing, ~60s)
just test-scenario-middle

# L4 hard (fault injection + recovery, ~45s)
just test-scenario-hard

# L1+L2 (CI default)
just test-scenario

# All scenario tests (nightly)
just test-scenario-all
```

Or directly with `go test`:

```bash
go test -tags scenario ./tests/scenario/ -run TestScenario_L1 -count=1 -v -timeout=120s
```

## Cross-repo scenario tests

phonewave is the orchestrator. Its scenario tests build and run all 4 tool binaries
(`sightjack`, `paintress`, `amadeus`, `phonewave`) plus `fake-claude` and `fake-gh`.

Each sibling repo (`sightjack/`, `paintress/`, `amadeus/`) also has its own
`tests/scenario/` that tests tool-specific behavior with phonewave routing.

To run all 4 repos:

```bash
for repo in phonewave sightjack paintress amadeus; do
  (cd /path/to/tap/$repo && just test-scenario)
done
```

## Test levels

| Level | Test | Focus |
|-------|------|-------|
| L1 | `TestScenario_L1_Minimal` | Single closed loop: spec → report → feedback |
| L2 | `TestScenario_L2_Small` | Multi-issue, priority ordering, retry/resolve |
| L3 | `TestScenario_L3_Middle` | Concurrent burst, convergence routing, fan-out |
| L4 | `TestScenario_L4_Hard` | Daemon restart, malformed D-Mail, recovery |
| - | `TestScenario_ClosedLoop_4Tool` | Full 4-tool closed loop (no injection) |

### Human-on-the-loop

phonewave is the D-Mail routing daemon and does not have interactive approval prompts.
Human-on-the-loop verification (`TestScenario_ApproveCmdPath`) is implemented in each
downstream tool's own `tests/scenario/approve_test.go`:

- `sightjack`: CmdApprover + go-expect PTY (convergence gate)
- `paintress`: CmdApprover + CmdNotifier (expedition gate)
- `amadeus`: CmdApprover + CmdNotifier (check gate)

phonewave's `TestScenario_ClosedLoop_4Tool` verifies that D-Mails route correctly
through the full 4-tool closed loop, which includes all downstream approval paths.

## Build tag

All scenario tests use `//go:build scenario`. They are excluded from regular
`go test ./...` runs and require `-tags scenario`.

## Troubleshooting

### `compile: version "go1.26.0" does not match go tool version "go1.19.3"`

GOROOT or GOTOOLDIR points to a different Go installation than the `go` binary in PATH.

```bash
# Diagnose — go tool compile -V reveals the actual compiler version
go version            # reports the go binary's own version
go tool compile -V    # reports the compiler from GOTOOLDIR (may differ!)
go env GOROOT         # must point to the same installation as go binary
go env GOTOOLDIR      # must be under GOROOT

# Fix (mise users)
unset GOROOT GOTOOLDIR
mise install go     # reinstall go 1.26
mise reshim         # regenerate shims

# Fix (manual)
unset GOROOT GOTOOLDIR
export PATH="$(go env GOROOT)/bin:$PATH"
```

All 4 repos pin `go = "1.26"` in `mise.toml` and `go 1.26` in `go.mod`.

The `check-go` guard in each repo's `justfile` compares `go version` with
`go tool compile -V` to detect GOROOT/GOTOOLDIR mismatches before running
scenario tests. All scenario recipes use `mise exec -- go test ...` to
ensure the mise-managed Go version is used regardless of PATH order.
