package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	cmd "github.com/hironow/phonewave/internal/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cmd.NewRootCommand().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
