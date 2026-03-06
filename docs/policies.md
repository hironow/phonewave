# Policy Engine

PolicyEngine dispatches domain events to registered handlers (best-effort, fire-and-forget).
Errors are logged but never propagated — `Dispatch()` always returns nil.

## Location

- Engine: `internal/usecase/policy.go`
- Handlers: `internal/usecase/policy_handlers.go`
- Policy definitions: `internal/domain/policy.go`
- Registration: `internal/usecase/daemon.go` → `registerDaemonPolicies()`

## Event → Handler Mapping

| Policy Name | WHEN [EVENT] | THEN [COMMAND] | Side Effects |
|---|---|---|---|
| DeliveryCompletedLogMetrics | delivery.completed | LogDeliveryMetrics | Log (Info) + Metrics |
| DeliveryFailedRecordError | delivery.failed | RecordDeliveryError | Log (Info) + Desktop Notify + Metrics |
| ErrorRetriedLogMetrics | error.retried | LogRetryMetrics | Log (Info) + Desktop Notify + Metrics |
| ScanCompletedLogMetrics | scan.completed | LogScanMetrics | Log (Info) + Desktop Notify + Metrics |

## Event Payload Format

All handlers use `map[string]string` unmarshaling from `event.Data`.

| Event | Payload Fields |
|---|---|
| delivery.completed | `kind` |
| delivery.failed | (none — uses event.Type) |
| error.retried | `name`, `kind` |
| scan.completed | `delivered`, `errors` |

## Dispatch Guarantee

Best-effort (at-most-once). Handler failures are silently logged.
No retry, no dead-letter queue, no error propagation to callers.
