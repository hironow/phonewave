package domain

import (
	"fmt"
	"time"
)

// RunDaemonCommand represents the intent to start the phonewave daemon.
// Independent of cobra — framework concerns are separated at the cmd layer.
type RunDaemonCommand struct {
	Verbose       bool
	DryRun        bool
	RetryInterval time.Duration
	MaxRetries    int
}

// Validate checks that the command has valid required fields.
func (c *RunDaemonCommand) Validate() []error {
	var errs []error
	if c.RetryInterval <= 0 {
		errs = append(errs, fmt.Errorf("RetryInterval must be positive"))
	}
	if c.MaxRetries < 0 {
		errs = append(errs, fmt.Errorf("MaxRetries must be non-negative"))
	}
	return errs
}

// AddRepoCommand represents the intent to add a repository to phonewave.
type AddRepoCommand struct {
	RepoPath string
}

// Validate checks that the command has valid required fields.
func (c *AddRepoCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	return errs
}

// RemoveRepoCommand represents the intent to remove a repository from phonewave.
type RemoveRepoCommand struct {
	RepoPath string
}

// Validate checks that the command has valid required fields.
func (c *RemoveRepoCommand) Validate() []error {
	var errs []error
	if c.RepoPath == "" {
		errs = append(errs, fmt.Errorf("RepoPath is required"))
	}
	return errs
}

// SyncCommand represents the intent to synchronize all watched repositories.
type SyncCommand struct{}

// Validate checks that the command has valid required fields.
func (c *SyncCommand) Validate() []error {
	return nil
}

// StatusCommand represents the intent to display phonewave daemon status.
type StatusCommand struct{}

// Validate checks that the command has valid required fields.
func (c *StatusCommand) Validate() []error {
	return nil
}
