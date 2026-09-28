package main

import (
	"github.com/spf13/cobra"
)

// newLogsCmd implements: campuscloud logs [service].
func newLogsCmd(a *app) *cobra.Command {
	var follow bool
	var tail int

	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "Show logs for the application services (nextcloud, mariadb, or all)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service := ""
			if len(args) == 1 {
				service = args[0]
			}
			return a.client.ComposeLogs(cmd.Context(), cmd.OutOrStdout(), service, tail, follow)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().IntVar(&tail, "tail", 100, "number of lines to show from the end of the logs")
	return cmd
}
