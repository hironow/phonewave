package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/eventsource"
	"github.com/spf13/cobra"
)

func newMetricsCmd() *cobra.Command {
	var window string
	var bucket string

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Output delivery health time-series as JSON",
		Long: `Read the JSONL event store and output a bucketed delivery health
time-series to stdout. Suitable for dashboard consumption, piping to jq,
or rendering with terminal tools (sampler, wtf).`,
		Example: `  # Default: last 7 days at hourly buckets
  phonewave metrics

  # Last 24 hours at 15-minute buckets
  phonewave metrics --window 24h --bucket 15m

  # Pipe to jq for totals only
  phonewave metrics | jq '.totals'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			windowDur, err := time.ParseDuration(window)
			if err != nil {
				return fmt.Errorf("invalid --window: %w", err)
			}
			bucketDur, err := time.ParseDuration(bucket)
			if err != nil {
				return fmt.Errorf("invalid --bucket: %w", err)
			}
			if bucketDur <= 0 {
				return fmt.Errorf("--bucket must be positive")
			}

			stateDir := configBase(cmd)
			store := eventsource.NewFileEventStore(
				eventsource.EventsDir(stateDir),
				nil,
			)

			now := time.Now().UTC()
			windowStart := now.Add(-windowDur)

			events, _, err := store.LoadSince(windowStart)
			if err != nil {
				return fmt.Errorf("load events: %w", err)
			}

			ts := domain.AggregateHealthTimeSeries(events, windowStart, bucketDur, now)
			ts.Window = windowDur.String()

			data, err := json.MarshalIndent(ts, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal JSON: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&window, "window", "168h", "Time window (e.g. 24h, 168h)")
	cmd.Flags().StringVar(&bucket, "bucket", "1h", "Bucket size (e.g. 15m, 1h)")

	return cmd
}
