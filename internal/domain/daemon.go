package domain

import "time"

// DaemonOptions configures the daemon behavior.
type DaemonOptions struct {
	Routes        []ResolvedRoute
	OutboxDirs    []string
	StateDir      string
	Verbose       bool
	DryRun        bool
	RetryInterval time.Duration // 0 = disabled (default)
	MaxRetries    int           // default 10
}
