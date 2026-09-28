package main

import (
	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/deployment"
)

// newDeployCmd implements FR-02: campuscloud deploy.
func newDeployCmd(a *app) *cobra.Command {
	var skipDoctor bool

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Validate the environment and deploy the Nextcloud + MariaDB stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := deployment.DefaultOptions()
			opts.SkipDoctor = skipDoctor
			return deployment.Deploy(cmd.Context(), a.cfg, a.client, cmd.OutOrStdout(), opts)
		},
	}

	cmd.Flags().BoolVar(&skipDoctor, "skip-doctor", false, "skip the environment check before deploying")
	return cmd
}
