package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/backup"
)

// newRestoreCmd implements FR-09: campuscloud restore [backup-id].
func newRestoreCmd(a *app) *cobra.Command {
	var latest bool

	cmd := &cobra.Command{
		Use:   "restore [backup-id]",
		Short: "Restore a backup: validate, stop, restore, restart, and health-check",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var backupID string
			switch {
			case len(args) == 1:
				backupID = args[0]
			case latest:
				backups, err := backup.List(a.cfg.Backup.Dir)
				if err != nil {
					return err
				}
				if len(backups) == 0 {
					return fmt.Errorf("no backups found in %s", a.cfg.Backup.Dir)
				}
				backupID = backups[0]
			default:
				return fmt.Errorf("specify a backup id (see `campuscloud backup list`) or pass --latest")
			}

			dir := backupID
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(a.cfg.Backup.Dir, backupID)
			}
			return backup.Restore(cmd.Context(), a.cfg, a.client, dir, cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&latest, "latest", false, "restore the most recent backup")
	return cmd
}
