package main

import (
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
			report := deployment.FullHealth(cmd.Context(), a.cfg, a.client)
			deployment.PrintReport(cmd.OutOrStdout(), report)
			if report.Overall == health.StatusFail {
				return errExitSilently
			}
			return nil
		},
	}
}
