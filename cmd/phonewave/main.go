package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cmd "github.com/hironow/phonewave/internal/cmd"
)

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cmd.NewRootCommand().ExecuteContext(ctx); err != nil {
		if !errors.Is(err, cmd.ErrUpdateAvailable) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}
	return 0
}
