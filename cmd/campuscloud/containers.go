package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newContainersCmd implements FR-05: campuscloud containers.
func newContainersCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "containers",
		Short: "List containers and their resource usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			containers, err := a.client.ComposePS(ctx)
			if err != nil {
				return err
			}
			if len(containers) == 0 {
				fmt.Fprintln(out, "No containers found. Has `campuscloud deploy` been run?")
				return nil
			}

			var names []string
			for _, c := range containers {
				names = append(names, c.Name)
			}

			stats, statsErr := a.client.Stats(ctx, names...)
			statsByName := make(map[string]string)
			for _, s := range stats {
				statsByName[s.Name] = fmt.Sprintf("cpu=%s mem=%s (%s) net=%s block=%s", s.CPUPerc, s.MemUsage, s.MemPerc, s.NetIO, s.BlockIO)
			}

			fmt.Fprintf(out, "%-28s %-10s %-10s %s\n", "NAME", "SERVICE", "STATE", "USAGE")
			for _, c := range containers {
				usage := statsByName[c.Name]
				if usage == "" {
					usage = "n/a (container not running)"
				}
				fmt.Fprintf(out, "%-28s %-10s %-10s %s\n", c.Name, c.Service, c.State, usage)
			}
			if statsErr != nil {
				fmt.Fprintf(out, "\nwarning: could not fetch live stats: %v\n", statsErr)
			}
			return nil
		},
	}
}
