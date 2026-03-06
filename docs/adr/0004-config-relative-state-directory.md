# 0004. Config-Relative State Directory

**Date:** 2026-02-23
**Status:** Accepted

## Context

phonewave stores runtime state (PID file, delivery log, error queue) in a
`.phonewave/` directory. The question is where this directory should be rooted.

Options considered:

- **CWD-relative**: `.phonewave/` in the current working directory. Simple but
  fragile — running from different directories creates orphaned state.
- **Home directory**: `~/.phonewave/`. Global but conflicts when managing
  multiple ecosystems.
- **Config-relative**: `.phonewave/` alongside `phonewave.yaml`. State lives
  where the configuration lives, regardless of CWD.

## Decision

Derive the state directory from the config file's location using `configBase(cmd)`:

1. **`configBase(cmd)`** returns `filepath.Dir(configPath(cmd))`, where
   `configPath` reads the `--config` flag.
2. **State directory** is `configBase(cmd) + "/.phonewave/"`.
3. **`EnsureStateDir`** creates the state directory if it does not exist,
   called by both `init` and `run` commands.
4. **Default config path** is `./phonewave.yaml`, making the default state
   directory `./.phonewave/` — matching user expectations for local usage.
5. **Custom config path** (`--config /path/to/phonewave.yaml`) automatically
   places state at `/path/to/.phonewave/` — no separate flag needed.

## Consequences

### Positive

- Running `phonewave run` from any directory produces consistent behavior
  as long as `--config` points to the same file
- No orphaned state directories from accidental CWD changes
- Multi-ecosystem setups work naturally with different config paths
- Single `--config` flag controls both config and state location

### Negative

- State directory location is implicit (derived, not configured directly)
- Users must understand that `--config` affects state directory placement
