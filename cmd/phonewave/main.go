package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	cmd "github.com/hironow/phonewave/internal/cmd"
)

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracer := cmd.InitTracer("phonewave", cmd.Version)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracer(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "tracer shutdown: %v\n", err)
		}
	}()

	if err := cmd.NewRootCommand().ExecuteContext(ctx); err != nil {
		if !errors.Is(err, cmd.ErrUpdateAvailable) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}
	return 0
}
