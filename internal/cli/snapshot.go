package cli

import (
	"fmt"

	"github.com/jorden-parker/claude-config-cli/internal/snapshot"
	"github.com/spf13/cobra"
)

func snapshotCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot FILE",
		Short: "Save settings and status-line preferences outside Claude's configuration",
		Long:  "Save existing user, global, project, and local settings and ccfg status-line preferences. Choose a new file outside .claude. Includes all settings, but not plugins, scripts, credentials stored elsewhere, or session history.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := snapshot.Save(args[0], cwd())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %d files to %s\n", n, args[0])
			return nil
		},
	}
}

func restoreCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "restore FILE",
		Short: "Restore settings from a snapshot",
		Long:  "Restore saved settings to the current home and the original project directory (override with -C). Existing files are refused unless --force is given. Files absent from the snapshot are left alone.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := snapshot.Restore(args[0], flagDir, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored %d files from %s\n", n, args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "replace existing files included in the snapshot")
	return cmd
}
