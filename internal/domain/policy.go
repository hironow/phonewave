package domain

// Policy represents an implicit reactive rule: WHEN [EVENT] THEN [COMMAND].
// See ADR S0014 for the POLICY pattern reference.
type Policy struct {
	Name    string    // unique identifier for the policy
	Trigger EventType // domain event that activates this policy
	Action  string    // description of the resulting command
}

// Policies registers all known implicit policies in phonewave.
var Policies = []Policy{
	{Name: "DeliveryCompletedLogMetrics", Trigger: EventDeliveryCompleted, Action: "LogDeliveryMetrics"},
	{Name: "DeliveryFailedRecordError", Trigger: EventDeliveryFailed, Action: "RecordDeliveryError"},
	{Name: "ErrorRetriedLogMetrics", Trigger: EventErrorRetried, Action: "LogRetryMetrics"},
	{Name: "ScanCompletedLogMetrics", Trigger: EventScanCompleted, Action: "LogScanMetrics"},
}
