package phonewave_test

import (
	"testing"
	"time"

	"github.com/hironow/phonewave"
)

func TestRunDaemonCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := phonewave.RunDaemonCommand{
		RetryInterval: 60 * time.Second,
		MaxRetries:    10,
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestRunDaemonCommand_Validate_InvalidRetryInterval(t *testing.T) {
	// given
	cmd := phonewave.RunDaemonCommand{
		RetryInterval: 0,
		MaxRetries:    10,
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for non-positive RetryInterval")
	}
}

func TestRunDaemonCommand_Validate_InvalidMaxRetries(t *testing.T) {
	// given
	cmd := phonewave.RunDaemonCommand{
		RetryInterval: 60 * time.Second,
		MaxRetries:    -1,
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for negative MaxRetries")
	}
}

func TestAddRepoCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := phonewave.AddRepoCommand{
		RepoPath: "/tmp/repo",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestAddRepoCommand_Validate_MissingRepoPath(t *testing.T) {
	// given
	cmd := phonewave.AddRepoCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing RepoPath")
	}
}

func TestRemoveRepoCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := phonewave.RemoveRepoCommand{
		RepoPath: "/tmp/repo",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestRemoveRepoCommand_Validate_MissingRepoPath(t *testing.T) {
	// given
	cmd := phonewave.RemoveRepoCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing RepoPath")
	}
}

func TestSyncCommand_Validate(t *testing.T) {
	// given — SyncCommand has no required fields
	cmd := phonewave.SyncCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestStatusCommand_Validate(t *testing.T) {
	// given — StatusCommand has no required fields
	cmd := phonewave.StatusCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}
