# phonewave self-improvement loop

## Purpose

`phonewave` is the delivery, routing, and replay-control side of the 4-tool loop.

It sits on the path:

`specification -> implementation -> verification -> correction`

and is responsible for moving corrective messages to the right inboxes without turning delivery into diagnosis logic.

## What this tool now does

`phonewave` now participates in the observable self-improvement loop mainly through observability and shared state vocabulary.

The current implementation does three things:

1. It delivers D-Mails without changing the producer-owned diagnosis.
2. It exposes corrective metadata on delivery spans.
3. It exposes retry backoff in the same provider-state vocabulary used by the other tools.

## Shared corrective metadata

Delivery spans can now expose metadata such as:

- `failure_type`
- `target_agent`
- `recurrence_count`
- `corrective_action`
- `retry_allowed`
- `escalation_reason`
- `correlation_id`
- `trace_id`
- `outcome`

This keeps cross-tool routing observable without forcing `phonewave` to re-diagnose the message.

## Corrective routing behavior

`phonewave` still acts as the courier, not the classifier.

The current slice does not move diagnosis rules into `phonewave`. Instead:

- producers decide corrective intent
- `phonewave` preserves routing behavior
- metadata and span attributes make the corrective thread observable end to end

That separation keeps routing simple and avoids hiding diagnosis inside delivery.

## Provider pause model

`phonewave` exposes retry backoff through the shared provider-state snapshot:

- `active`
- `waiting`
- `degraded`
- `paused`

In the current implementation, backoff mainly maps to:

- `active` when retry pressure has reset
- `waiting` when delivery retry backoff is in effect

This keeps delivery waiting state compatible with the same vocabulary used by provider-facing tools.

## Current scope

What is in:

- delivery-span visibility for corrective metadata
- shared provider-state snapshot for retry backoff

What is not in yet:

- diagnosis-aware routing inside `phonewave`
- learned replay policy
- a central improvement controller

