package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newConfigCmd implements FR-10: campuscloud config — shows the effective
// configuration (YAML defaults + config.yaml + environment overrides) and
// can validate that required secrets are set.
func newConfigCmd(a *app) *cobra.Command {
	var validate bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show the effective configuration, or validate required settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg := a.cfg

			fmt.Fprintf(out, "project.name:              %s\n", cfg.Project.Name)
			fmt.Fprintf(out, "network.name:              %s\n", cfg.Network.Name)
			fmt.Fprintf(out, "volumes.nextcloud_data:    %s\n", cfg.Volumes.NextcloudData)
			fmt.Fprintf(out, "volumes.mariadb_data:      %s\n", cfg.Volumes.MariaDBData)
			fmt.Fprintf(out, "compose.file:              %s\n", cfg.Compose.File)
			fmt.Fprintf(out, "nextcloud.http_port:       %d\n", cfg.Nextcloud.HTTPPort)
			fmt.Fprintf(out, "nextcloud.container_name:  %s\n", cfg.Nextcloud.ContainerName)
			fmt.Fprintf(out, "mariadb.container_name:    %s\n", cfg.MariaDB.ContainerName)
			fmt.Fprintf(out, "monitoring.thresholds:     warning=%.0f%% high=%.0f%% critical=%.0f%%\n",
				cfg.Monitoring.Thresholds.Warning, cfg.Monitoring.Thresholds.High, cfg.Monitoring.Thresholds.Critical)
			fmt.Fprintf(out, "backup.dir:                %s\n", cfg.Backup.Dir)
			fmt.Fprintf(out, "backup.retain:             %d\n", cfg.Backup.Retain)
			fmt.Fprintf(out, "db.database:               %s\n", cfg.DB.Database)
			fmt.Fprintf(out, "db.user:                   %s\n", cfg.DB.User)
			fmt.Fprintf(out, "db.root_password:          %s\n", redact(cfg.DB.RootPassword))
			fmt.Fprintf(out, "db.password:               %s\n", redact(cfg.DB.Password))
			fmt.Fprintf(out, "admin.user:                %s\n", cfg.AdminUser)
			fmt.Fprintf(out, "admin.password:            %s\n", redact(cfg.AdminPassword))

			if validate {
				if err := cfg.Validate(); err != nil {
					return err
				}
				fmt.Fprintln(out, "\nConfiguration is valid.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&validate, "validate", false, "also validate that required secrets are set")
	return cmd
}

func redact(secret string) string {
	if secret == "" {
		return "(not set)"
	}
	return "********"
}
