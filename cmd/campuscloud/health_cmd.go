package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/deployment"
	"github.com/2005Hari/campuscloud/internal/health"
)

// newHealthCmd implements FR-07: campuscloud health — checks Docker,
// Nextcloud, MariaDB, database connectivity, storage, network, and HTTP
// availability.
func newHealthCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Run a full health check: Docker, Nextcloud, MariaDB, storage, network, and HTTP",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := a.cfg
			checkers := []health.Checker{
				health.DockerDaemonCheck(a.client),
				health.NetworkCheck(cfg.Network.Name, a.client),
				health.ContainerRunningCheck(cfg.Nextcloud.ContainerName, a.client),
				health.ContainerHealthCheck(cfg.Nextcloud.ContainerName, a.client),
				health.ContainerRunningCheck(cfg.MariaDB.ContainerName, a.client),
				health.ContainerHealthCheck(cfg.MariaDB.ContainerName, a.client),
				health.ComposeExecCheck("database-connectivity", a.client, cfg.MariaDBService(),
					"sh", "-c", `MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqladmin ping -uroot`),
				health.StorageCheck(cfg.Backup.Dir, cfg.Monitoring.Thresholds),
				health.HTTPCheck("nextcloud-http", fmt.Sprintf("http://localhost:%d%s", cfg.Nextcloud.HTTPPort, cfg.Nextcloud.HealthPath), 5*time.Second),
			}

			report := health.Run(cmd.Context(), checkers)
			deployment.PrintReport(cmd.OutOrStdout(), report)
			if report.Overall == health.StatusFail {
				return errExitSilently
			}
			return nil
		},
	}
}
