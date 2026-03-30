package domain

import "time"

// DefaultIdleTimeout is the default idle timeout for the daemon.
// The daemon exits cleanly when no activity occurs for this duration.
const DefaultIdleTimeout = 30 * time.Minute

// maxIdleTimeout is the safety cap applied when IdleTimeout=0 (no timeout).
// Prevents indefinite daemon hang in unattended environments (CI/CD).
const maxIdleTimeout = 24 * time.Hour

// EffectiveIdleTimeout returns the effective idle timeout duration,
// applying the safety cap when timeout is 0.
// Returns 0 when idle timeout is disabled (negative value).
func EffectiveIdleTimeout(timeout time.Duration) time.Duration {
	if timeout < 0 {
		return 0 // disabled
	}
	if timeout == 0 {
		return maxIdleTimeout
	}
	return timeout
}

// DaemonOptions configures the daemon behavior.
type DaemonOptions struct {
	Routes        []ResolvedRoute
	OutboxDirs    []string
	StateDir      string
	Verbose       bool
	DryRun        bool
	RetryInterval time.Duration // 0 = disabled (default)
	MaxRetries    int           // default 10
	IdleTimeout   time.Duration // >0 = exit after N idle, 0 = 24h safety cap, <0 = disabled
}
