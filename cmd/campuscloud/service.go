package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newStartCmd, newStopCmd, and newRestartCmd implement FR-03 (Service
// Management): campuscloud start|stop|restart.

func newStartCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the CampusCloud application services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.client.ComposeStart(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Services started.")
			return nil
		},
	}
}

func newStopCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the CampusCloud application services (data is preserved)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.client.ComposeStop(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Services stopped.")
			return nil
		},
	}
}

func newRestartCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the CampusCloud application services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.client.ComposeRestart(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Services restarted.")
			return nil
		},
	}
}
