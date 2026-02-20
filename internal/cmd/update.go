package cmd

import (
	"errors"
	"fmt"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

// ErrUpdateAvailable is returned by update --check when a newer version exists.
// Callers can check for this sentinel to distinguish "update available" (exit 1)
// from real errors.
var ErrUpdateAvailable = errors.New("update available")

const repoSlug = "hironow/phonewave"

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update phonewave to the latest version",
		Long:  "Check for and install the latest version of phonewave from GitHub releases.",
		Args:  cobra.NoArgs,
		Example: `  # Check for updates without installing
  phonewave update --check

  # Update to latest version
  phonewave update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			checkOnly, _ := cmd.Flags().GetBool("check")

			latest, found, err := selfupdate.DetectLatest(cmd.Context(), selfupdate.ParseSlug(repoSlug))
			if err != nil {
				return fmt.Errorf("detect latest version: %w", err)
			}
			if !found {
				fmt.Fprintln(cmd.OutOrStdout(), "No release found for this platform.")
				return nil
			}

			currentVer := Version
			if currentVer == "dev" {
				fmt.Fprintf(cmd.OutOrStdout(), "Running dev build. Latest release: %s\n", latest.Version())
				if checkOnly {
					return ErrUpdateAvailable
				}
			} else if latest.LessOrEqual(currentVer) {
				fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (%s).\n", currentVer)
				return nil
			}

			if checkOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s → %s\n", currentVer, latest.Version())
				return ErrUpdateAvailable
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updating to %s ...\n", latest.Version())
			if err := selfupdate.UpdateTo(cmd.Context(), latest.AssetURL, latest.AssetName, ""); err != nil {
				return fmt.Errorf("update: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s\n", latest.Version())
			return nil
		},
	}

	cmd.Flags().Bool("check", false, "Check for updates without installing")

	return cmd
}
