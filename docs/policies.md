# Policy Engine

PolicyEngine dispatches domain events to registered handlers (best-effort, fire-and-forget).
Errors are logged but never propagated — `Dispatch()` always returns nil.

## Location

- Engine: `internal/usecase/policy.go`
- Handlers: `internal/usecase/policy_handlers.go`
- Registration: `internal/usecase/daemon.go` → `registerDaemonPolicies()`

## Event → Handler Mapping

| Policy Name | WHEN [EVENT] | THEN [COMMAND] | Side Effects |
|---|---|---|---|
| DeliveryCompletedLogMetrics | delivery.completed | LogDeliveryMetrics | Log (Info) + Metrics |
| DeliveryFailedRecordError | delivery.failed | RecordDeliveryError | Log (Info) + Desktop Notify + Metrics |
| ErrorRetriedLogMetrics | error.retried | LogRetryMetrics | Log (Info) + Desktop Notify + Metrics |
| ScanCompletedLogMetrics | scan.completed | LogScanMetrics | Log (Info) + Desktop Notify + Metrics |

## Event Payload Format

| Event | Payload Type | Fields |
|---|---|---|
| delivery.completed | `domain.DeliveryCompletedPayload` | `Path`, `Kind` |
| delivery.failed | `domain.DeliveryFailedPayload` | `Path`, `Kind`, `Error` |
| error.retried | `domain.ErrorRetriedPayload` | `Name`, `Kind` |
| scan.completed | `domain.ScanCompletedPayload` | `Outbox`, `Delivered`, `Failed` |

## Dispatch Guarantee

Best-effort (at-most-once). Handler failures are silently logged.
No retry, no dead-letter queue, no error propagation to callers.

## Skeleton Handlers

DeliveryCompletedLogMetrics is an observation-only placeholder
(Info log + Metrics, no notification).
