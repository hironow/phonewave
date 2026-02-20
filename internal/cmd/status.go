package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon and delivery status",
		Long:  "Show daemon state, uptime, watched directories, route count, error queue, and 24h delivery statistics.",
		Args:  cobra.NoArgs,
		Example: `  phonewave status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := filepath.Join(".", phonewave.ConfigFile)
			cfg, err := phonewave.LoadConfig(configPath)
			if err != nil {
				phonewave.LogInfo("Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			stateDir := filepath.Join(".", phonewave.StateDir)
			status := phonewave.Status(cfg, stateDir)

			fmt.Fprintf(os.Stderr, "phonewave status:\n")
			if status.DaemonRunning {
				fmt.Fprintf(os.Stderr, "  Daemon:    running (PID %d)\n", status.DaemonPID)
			} else {
				fmt.Fprintf(os.Stderr, "  Daemon:    stopped\n")
			}
			if status.Uptime > 0 {
				fmt.Fprintf(os.Stderr, "  Uptime:    %s\n", status.Uptime.Truncate(time.Second))
			}
			fmt.Fprintf(os.Stderr, "  Watching:  %d outbox directories across %d repositories\n", status.OutboxCount, status.RepoCount)
			fmt.Fprintf(os.Stderr, "  Routes:    %d\n", status.RouteCount)
			fmt.Fprintf(os.Stderr, "  Pending:   %d items in error queue\n", status.PendingErrors)
			fmt.Fprintf(os.Stderr, "  Last 24h:  %d delivered, %d failed, %d retried\n",
				status.DeliveredCount24h, status.FailedCount24h, status.RetriedCount24h)

			return nil
		},
	}
}
