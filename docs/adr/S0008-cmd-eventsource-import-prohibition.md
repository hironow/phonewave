# S0008. cmd Layer Prohibition of Direct eventsource Import

**Date:** 2026-02-28
**Status:** Accepted

## Context

The 3-layer architecture (ref: S0004) defines a strict dependency direction:

```
internal/cmd -> internal/session -> internal/eventsource -> root
```

In practice, several `internal/cmd/` files across sightjack (6 files), paintress
(1 file), and amadeus (6 files) directly import `internal/eventsource/` to
construct FileEventStore instances and call lifecycle functions (Open, Close,
EventsDir). This creates a shortcut around the session layer.

The question is whether to permit these direct imports for lifecycle management
(Open/Close/EventsDir) or to enforce the strict layer direction by routing all
eventsource access through the session layer.

## Decision

Direct import of `internal/eventsource/` from `internal/cmd/` is **prohibited**.

All eventsource access from the cmd layer must go through the session layer.
The session layer provides:

1. **EventStore construction:** A factory or initialization function that creates
   and returns the EventStore, abstracting the concrete implementation.
2. **Lifecycle management:** Open/Close operations wrapped in session-level
   functions that can be called from cmd layer code.
3. **Path derivation:** EventsDir and related path functions exposed through
   the session layer or the root package.

This aligns with the port adapter pattern where the session layer is the adapter
for all infrastructure concerns.

## Consequences

### Positive
- Strict adherence to S0004 layer architecture
- Session layer becomes the single point of control for eventsource lifecycle
- Easier to swap eventsource implementations (e.g., for testing)
- semgrep rules can enforce this without exceptions

### Negative
- 13 files require refactoring (6 sightjack + 1 paintress + 6 amadeus)
- Session layer gains thin wrapper functions that may feel like boilerplate
- Slightly more indirection for simple EventStore construction

### Neutral
- Root package EventStore interface and EventsDir path function remain available
  to all layers (root is importable by all per S0004)
- The eventsource package itself does not change
- This decision applies to all 4 tools uniformly
