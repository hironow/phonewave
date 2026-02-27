# S0007. Config-Relative State Directory

**Date:** 2026-02-27
**Status:** Accepted
**Supersedes:** phonewave ADR 0009 (tool-specific)

## Context

Each tool stores runtime state in a tool-specific directory:

- phonewave: `.phonewave/` (PID file, delivery log, error queue)
- sightjack: `.siren/` (scan results, wave files, SQLite)
- paintress: `.expedition/` (journal, flags, SQLite)
- amadeus: `.gate/` (check state, sync state, SQLite)

The question is where these directories should be rooted.

Options considered:

- **CWD-relative**: State dir in the current working directory. Simple but
  fragile — running from different directories creates orphaned state.
- **Home directory**: Global but conflicts when managing multiple projects.
- **Config-relative**: State dir alongside the config file. State lives where
  the configuration lives, regardless of CWD.

## Decision

Derive the state directory from the config file's or target directory's location:

1. **Tools with `--config` flag** (phonewave, sightjack): State directory is
   derived from `filepath.Dir(configPath)`.
2. **Tools with path argument** (paintress, amadeus): State directory is placed
   inside the target repository directory.
3. **Default behavior**: When no explicit path is given, use CWD as the base,
   making the default state directory `./.<toolname>/` — matching user
   expectations for local usage.
4. **State directory creation**: `init` and `run` commands ensure the state
   directory exists. `clean` removes it.

## Consequences

### Positive

- Running tools from any directory produces consistent behavior as long as
  the config/target path is the same
- No orphaned state directories from accidental CWD changes
- Multi-project setups work naturally with different config/target paths
- Single path controls both config and state location

### Negative

- State directory location is implicit (derived, not configured directly)
- Users must understand that config path affects state directory placement
- amadeus currently uses `os.Getwd()` instead of path arguments (to be
  addressed in K3a path argument unification)
