package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set at build time via:
//
//	go build -ldflags "-X main.version=1.0.0"
//
// and defaults to "dev" for local, non-release builds.
var version = "dev"

// newVersionCmd implements: campuscloud version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the campuscloud CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "campuscloud version %s\n", version)
			return nil
		},
	}
}
