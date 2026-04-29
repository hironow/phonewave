package domain

import (
	"fmt"
	"time"
)

// TrackingMode determines the issue tracking backend.
type TrackingMode string

const (
	ModeWave   TrackingMode = "wave"
	ModeLinear TrackingMode = "linear"
)

func NewTrackingMode(linear bool) TrackingMode {
	if linear {
		return ModeLinear
	}
	return ModeWave
}

func (m TrackingMode) IsLinear() bool { return m == ModeLinear }
func (m TrackingMode) IsWave() bool   { return m == ModeWave }
func (m TrackingMode) String() string { return string(m) }

// RepoPath is an always-valid, non-empty repository path.
type RepoPath struct{ v string } // nosemgrep: structure.multiple-exported-structs-go -- primitives family (RepoPath/ConfigPath/NonEmptyRepoPaths/RetryInterval/MaxRetries) is cohesive newtype primitive set [permanent]

func NewRepoPath(raw string) (RepoPath, error) {
	if raw == "" {
		return RepoPath{}, fmt.Errorf("RepoPath is required")
	}
	return RepoPath{v: raw}, nil
}

func (r RepoPath) String() string { return r.v }

// ConfigPath is an always-valid, non-empty configuration file path.
type ConfigPath struct{ v string } // nosemgrep: structure.multiple-exported-structs-go -- primitives family; see RepoPath [permanent]

func NewConfigPath(raw string) (ConfigPath, error) {
	if raw == "" {
		return ConfigPath{}, fmt.Errorf("ConfigPath is required")
	}
	return ConfigPath{v: raw}, nil
}

func (c ConfigPath) String() string { return c.v }

// NonEmptyRepoPaths guarantees at least one RepoPath.
type NonEmptyRepoPaths struct{ v []RepoPath } // nosemgrep: structure.multiple-exported-structs-go -- primitives family; see RepoPath [permanent]

func NewNonEmptyRepoPaths(paths []RepoPath) (NonEmptyRepoPaths, error) {
	if len(paths) == 0 {
		return NonEmptyRepoPaths{}, fmt.Errorf("at least one RepoPath is required")
	}
	return NonEmptyRepoPaths{v: paths}, nil
}

func (n NonEmptyRepoPaths) Paths() []RepoPath { return n.v }

func (n NonEmptyRepoPaths) Strings() []string {
	out := make([]string, len(n.v))
	for i, p := range n.v {
		out[i] = p.String()
	}
	return out
}

// RetryInterval is an always-valid, positive duration for retry intervals.
type RetryInterval struct{ v time.Duration } // nosemgrep: structure.multiple-exported-structs-go -- primitives family; see RepoPath [permanent]

func NewRetryInterval(d time.Duration) (RetryInterval, error) {
	if d <= 0 {
		return RetryInterval{}, fmt.Errorf("RetryInterval must be positive")
	}
	return RetryInterval{v: d}, nil
}

func (r RetryInterval) Duration() time.Duration { return r.v }

// MaxRetries is an always-valid, non-negative maximum retry count.
type MaxRetries struct{ v int }

func NewMaxRetries(n int) (MaxRetries, error) {
	if n < 0 {
		return MaxRetries{}, fmt.Errorf("MaxRetries must be non-negative")
	}
	return MaxRetries{v: n}, nil
}

func (m MaxRetries) Int() int { return m.v }
