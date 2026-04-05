# What / Why / How Conformance

This is the single source of truth for phonewave's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | File-based message courier daemon that routes D-Mails between AI agent tool repositories |
| **Why** | Enable inter-tool communication without shared databases or direct API coupling |
| **How** | fsnotify watch on outbox directories → YAML frontmatter routing → atomic inbox delivery → SQLite error queue with retry |
| **Input** | D-Mail `.md` files written to outbox directories by other tools |
| **Output** | D-Mail `.md` files delivered to inbox directories of consuming tools |
| **Telemetry** | OTel spans: `phonewave.run`, `startup_scan`, `handle_event`, `deliver_data` |
| **External Systems** | File system (fsnotify), OTel exporter (Jaeger/Weave) |

## Layer Architecture

```
cmd              --> usecase, session, harness, usecase/port, platform, domain  (composition root)
usecase          --> usecase/port, harness, domain                              (output port only)
usecase/port     --> domain (+ stdlib)                                          (interface contracts)
session          --> eventsource, usecase/port, harness, platform, domain       (adapter impl)
harness          --> domain                                                     (policy-first seam / thin facade)
eventsource      --> domain                                                     (event persistence adapter)
platform         --> domain (+ stdlib)                                          (cross-cutting infra)
domain           --> (nothing internal, stdlib only)                            (pure types/logic)
```

### Harness Layer

`phonewave` is the **policy-first / harness-thin** exception in the shared harness contract.

- `internal/harness` is the stable facade seam for future extractions.
- The current deterministic routing and retry decisions still live primarily in `internal/usecase/policy.go`, `internal/domain/*`, and selected `internal/session/*` adapters.
- New extraction work must still converge on the same dependency rules as the other 3 tools: external callers should import the facade, and policy logic should remain more independent than verifier/filter-like code.

`eventsource` is the event persistence adapter based on the [AWS Event Sourcing pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/event-sourcing.html).
Its responsibility is limited to append, load, and replay of domain events.
Event store implementation MUST NOT exist outside `internal/eventsource`.
`session` uses `eventsource` as a client but does not implement event persistence itself.

Key constraints enforced by semgrep (ERROR severity):

- `usecase --> session` PROHIBITED (must use output port interfaces)
- `cmd --> eventsource` PROHIBITED (ADR S0008)
- `domain` has no I/O, no `context.Context`
- `domain --> harness` PROHIBITED
- `eventsource --> harness` PROHIBITED
- external callers must use the `harness` facade instead of reaching into sub-packages when phonewave grows beyond the current thin seam

Ref: `.semgrep/layers.yaml`, `.semgrep/layers-harness.yaml`, `refs/opsx/semgrep-layer-contract.md`, ADR S0007

## Domain Primitives & Parse-Don't-Validate

Domain command types use the Parse-Don't-Validate pattern:

- Domain primitives (`RepoPath`, `ConfigPath`, `NonEmptyRepoPaths`, `RetryInterval`, `MaxRetries`) validate in `New*()` constructors — invalid values are rejected at parse time
- Command types use unexported fields with `New*Command()` constructors that accept only pre-validated primitives
- Commands are always-valid by construction — no `Validate() []error` methods exist
- Usecase layer receives always-valid commands with no validation boilerplate
- Semgrep rule `domain-no-validate-method` prevents reintroduction of `Validate() []error`

Ref: `.semgrep/layers.yaml`, ADR S0029

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
