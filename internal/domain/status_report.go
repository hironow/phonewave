package domain

import "time"

// StatusReport holds daemon and ecosystem status information.
type StatusReport struct {
	DaemonRunning     bool
	DaemonPID         int
	OutboxCount       int
	RouteCount        int
	RepoCount         int
	PendingErrors     int
	Uptime            time.Duration
	DeliveredCount24h int
	FailedCount24h    int
	RetriedCount24h   int
	SuccessRate24h    float64
}
