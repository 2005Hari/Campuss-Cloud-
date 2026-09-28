package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/monitoring"
)

// newStatusCmd implements FR-04: campuscloud status.
func newStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show container state, CPU, memory, storage, uptime, and health",
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

			fmt.Fprintln(out, "Containers:")
			for _, c := range containers {
				health := c.Health
				if health == "" {
					health = "n/a"
				}
				fmt.Fprintf(out, "  %-28s %-10s %-22s health=%s\n", c.Name, c.State, c.Status, health)
			}

			t := a.cfg.Monitoring.Thresholds

			fmt.Fprintln(out)
			cpu, err := monitoring.HostCPU(300*time.Millisecond, t)
			if err != nil {
				fmt.Fprintf(out, "CPU:     unavailable (%v)\n", err)
			} else {
				fmt.Fprintf(out, "CPU:     %.1f%% [%s]\n", cpu.Percent, cpu.State)
			}

			mem, err := monitoring.HostMemory(t)
			if err != nil {
				fmt.Fprintf(out, "Memory:  unavailable (%v)\n", err)
			} else {
				fmt.Fprintf(out, "Memory:  %.1f%% used (%s / %s) [%s]\n", mem.Percent, monitoring.FormatBytes(mem.Used), monitoring.FormatBytes(mem.Total), mem.State)
			}

			storage, err := monitoring.HostStorage(a.cfg.Backup.Dir, t)
			if err != nil {
				fmt.Fprintf(out, "Storage: unavailable (%v)\n", err)
			} else {
				fmt.Fprintf(out, "Storage: %.1f%% used (%s / %s) [%s]\n", storage.Percent, monitoring.FormatBytes(storage.Used), monitoring.FormatBytes(storage.Total), storage.State)
			}

			return nil
		},
	}
}
