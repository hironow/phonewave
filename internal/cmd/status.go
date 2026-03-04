package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/platform"
	"github.com/hironow/phonewave/internal/usecase"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [path]",
		Short: "Show daemon and delivery status",
		Long:  "Show daemon state, uptime, watched directories, route count, error queue, and 24h delivery statistics.",
		Args:  cobra.MaximumNArgs(1),
		Example: `  phonewave status
  phonewave status /path/to/project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)

			base, err := resolveBaseDir(cmd, args)
			if err != nil {
				return err
			}
			cfgPath := filepath.Join(base, domain.ConfigFile)
			stateDir := filepath.Join(base, domain.StateDir)
			status, err := usecase.GetStatus(cfgPath, stateDir)
			if err != nil {
				logger.Info("Run 'phonewave init' first")
				return err
			}

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
			fmt.Fprintf(w, "  Success:   %s\n",
				domain.FormatSuccessRate(status.SuccessRate24h, status.DeliveredCount24h,
					status.DeliveredCount24h+status.FailedCount24h))

			return nil
		},
	}
}
