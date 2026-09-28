package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/backup"
)

// newBackupCmd implements FR-08: campuscloud backup.
func newBackupCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up the database, configuration, and user files",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := backup.Create(cmd.Context(), a.cfg, a.client, cmd.OutOrStdout())
			return err
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available backups, most recent first",
		RunE: func(cmd *cobra.Command, args []string) error {
			backups, err := backup.List(a.cfg.Backup.Dir)
			if err != nil {
				return err
			}
			if len(backups) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No backups found.")
				return nil
			}
			for _, b := range backups {
				fmt.Fprintln(cmd.OutOrStdout(), b)
			}
			return nil
		},
	})

	return cmd
}
