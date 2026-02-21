package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hironow/phonewave"
	cmd "github.com/hironow/phonewave/internal/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracer := phonewave.InitTracer("phonewave", cmd.Version)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracer(ctx); err != nil {
			phonewave.LogWarn("tracer shutdown: %v", err)
		}
	}()

	if err := cmd.NewRootCommand().ExecuteContext(ctx); err != nil {
		if errors.Is(err, cmd.ErrUpdateAvailable) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
