package domain

import "time"

// InitCommand represents the intent to initialize phonewave configuration.
type InitCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- command family (InitCommand/RunDaemonCommand/AddRepoCommand/RemoveRepoCommand/SyncCommand/StatusCommand) is cohesive command-value-object set; all represent domain intents [permanent]
	repoPaths  NonEmptyRepoPaths
	configPath ConfigPath
}

func NewInitCommand(repoPaths NonEmptyRepoPaths, configPath ConfigPath) InitCommand {
	return InitCommand{repoPaths: repoPaths, configPath: configPath}
}

func (c InitCommand) RepoPaths() NonEmptyRepoPaths { return c.repoPaths }
func (c InitCommand) ConfigPath() ConfigPath       { return c.configPath }

// RunDaemonCommand represents the intent to start the phonewave daemon.
type RunDaemonCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- command family; see InitCommand [permanent]
	verbose       bool
	dryRun        bool
	retryInterval RetryInterval
	maxRetries    MaxRetries
	idleTimeout   time.Duration
}

func NewRunDaemonCommand(verbose, dryRun bool, retryInterval RetryInterval, maxRetries MaxRetries, idleTimeout time.Duration) RunDaemonCommand {
	return RunDaemonCommand{
		verbose:       verbose,
		dryRun:        dryRun,
		retryInterval: retryInterval,
		maxRetries:    maxRetries,
		idleTimeout:   idleTimeout,
	}
}

func (c RunDaemonCommand) Verbose() bool                { return c.verbose }
func (c RunDaemonCommand) DryRun() bool                 { return c.dryRun }
func (c RunDaemonCommand) RetryInterval() RetryInterval { return c.retryInterval }
func (c RunDaemonCommand) RetryDuration() time.Duration { return c.retryInterval.Duration() }
func (c RunDaemonCommand) MaxRetries() MaxRetries       { return c.maxRetries }
func (c RunDaemonCommand) MaxRetriesInt() int           { return c.maxRetries.Int() }
func (c RunDaemonCommand) IdleTimeout() time.Duration   { return c.idleTimeout }

// AddRepoCommand represents the intent to add a repository to phonewave.
type AddRepoCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- command family; see InitCommand [permanent]
	repoPath RepoPath
}

func NewAddRepoCommand(repoPath RepoPath) AddRepoCommand {
	return AddRepoCommand{repoPath: repoPath}
}

func (c AddRepoCommand) RepoPath() RepoPath { return c.repoPath }

// RemoveRepoCommand represents the intent to remove a repository from phonewave.
type RemoveRepoCommand struct { // nosemgrep: structure.multiple-exported-structs-go -- command family; see InitCommand [permanent]
	repoPath RepoPath
}

func NewRemoveRepoCommand(repoPath RepoPath) RemoveRepoCommand {
	return RemoveRepoCommand{repoPath: repoPath}
}

func (c RemoveRepoCommand) RepoPath() RepoPath { return c.repoPath }

// SyncCommand represents the intent to synchronize all watched repositories.
type SyncCommand struct{} // nosemgrep: structure.multiple-exported-structs-go -- command family; see InitCommand [permanent]

// StatusCommand represents the intent to display phonewave daemon status.
type StatusCommand struct{}
