package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/phonewave"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove state directory and config file",
		Long:  "Delete .phonewave/ and phonewave.yaml to reset to a clean state.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			base := configBase(cmd)
			stateDir := filepath.Join(base, phonewave.StateDir)
			cfgPath := configPath(cmd)

			stateDirExists := dirExists(stateDir)
			cfgExists := fileExists(cfgPath)

			if !stateDirExists && !cfgExists {
				fmt.Fprintf(cmd.ErrOrStderr(), "Nothing to clean at %s\n", base)
				return nil
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "The following will be deleted:\n")
				if stateDirExists {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s/\n", stateDir)
				}
				if cfgExists {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", cfgPath)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "\nDelete? [y/N]: ")
				var answer string
				fmt.Fscanln(cmd.InOrStdin(), &answer)
				if answer != "y" && answer != "Y" {
					fmt.Fprintf(cmd.ErrOrStderr(), "Aborted.\n")
					return nil
				}
			}

			if stateDirExists {
				if err := os.RemoveAll(stateDir); err != nil {
					return fmt.Errorf("remove %s: %w", stateDir, err)
				}
			}
			if cfgExists {
				if err := os.Remove(cfgPath); err != nil {
					return fmt.Errorf("remove %s: %w", cfgPath, err)
				}
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Cleaned %s\n", base)
			return nil
		},
	}
	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	return cmd
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
