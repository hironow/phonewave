package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Args:  cobra.NoArgs,
		Example: `  phonewave version
  phonewave version --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")

			info := map[string]string{
				"version": Version,
				"commit":  Commit,
				"date":    Date,
				"go":      runtime.Version(),
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			}

			if jsonFlag {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "phonewave %s\n", Version)
			fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", Commit)
			fmt.Fprintf(cmd.OutOrStdout(), "  built:  %s\n", Date)
			fmt.Fprintf(cmd.OutOrStdout(), "  go:     %s\n", runtime.Version())
			fmt.Fprintf(cmd.OutOrStdout(), "  os:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output version info as JSON")

	return cmd
}
