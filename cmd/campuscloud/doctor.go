package main

import (
	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/deployment"
	"github.com/2005Hari/campuscloud/internal/health"
)

// newDoctorCmd implements FR-01: campuscloud doctor.
func newDoctorCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Verify Docker, Docker Compose, the daemon, RAM, and storage are ready for deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := deployment.Doctor(cmd.Context(), a.cfg, a.client)
			deployment.PrintReport(cmd.OutOrStdout(), report)
			if report.Overall == health.StatusFail {
				return errExitSilently
			}
			return nil
		},
	}
}
