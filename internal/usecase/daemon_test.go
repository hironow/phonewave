package usecase

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/hironow/phonewave"
)

func TestSetupAndRunDaemon_InvalidCommand(t *testing.T) {
	// given: RetryInterval <= 0
	cmd := phonewave.RunDaemonCommand{
		RetryInterval: 0,
		MaxRetries:    10,
	}
	logger := phonewave.NewLogger(io.Discard, false)

	// when
	err := SetupAndRunDaemon(context.Background(), cmd, "/nonexistent/config.yaml", "/tmp", logger)

	// then
	if err == nil {
		t.Fatal("expected validation error for zero RetryInterval")
	}
}

func TestSetupAndRunDaemon_MissingConfig(t *testing.T) {
	// given: valid command but nonexistent config
	cmd := phonewave.RunDaemonCommand{
		RetryInterval: 60 * time.Second,
		MaxRetries:    10,
	}
	logger := phonewave.NewLogger(io.Discard, false)

	// when
	err := SetupAndRunDaemon(context.Background(), cmd, "/nonexistent/config.yaml", "/tmp", logger)

	// then
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}
