package cmd

import (
	"fmt"
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
			cfgPath := configPath(cmd)
			cfg, err := phonewave.LoadConfig(cfgPath)
			if err != nil {
				phonewave.LogInfo("Run 'phonewave init' first")
				return fmt.Errorf("load config: %w", err)
			}

			stateDir := filepath.Join(".", phonewave.StateDir)
			status := phonewave.Status(cfg, stateDir)

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "phonewave status:\n")
			if status.DaemonRunning {
				fmt.Fprintf(w, "  Daemon:    running (PID %d)\n", status.DaemonPID)
			} else {
				fmt.Fprintf(w, "  Daemon:    stopped\n")
			}
			if status.Uptime > 0 {
				fmt.Fprintf(w, "  Uptime:    %s\n", status.Uptime.Truncate(time.Second))
			}
			fmt.Fprintf(w, "  Watching:  %d outbox directories across %d repositories\n", status.OutboxCount, status.RepoCount)
			fmt.Fprintf(w, "  Routes:    %d\n", status.RouteCount)
			fmt.Fprintf(w, "  Pending:   %d items in error queue\n", status.PendingErrors)
			fmt.Fprintf(w, "  Last 24h:  %d delivered, %d failed, %d retried\n",
				status.DeliveredCount24h, status.FailedCount24h, status.RetriedCount24h)

			return nil
		},
	}
}
