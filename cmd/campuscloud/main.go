// Command campuscloud is the CampusCloud administration CLI: it automates
// deployment, monitoring, configuration, backup, and recovery of the
// Nextcloud + MariaDB private-cloud stack described in the CampusCloud PRD.
package main

import (
	"errors"
	"fmt"
	"os"
)

// errExitSilently signals that a command already printed a full,
// human-readable explanation of its failure (e.g. a health report) and
// main should just exit non-zero without adding a redundant "Error: ..." line.
var errExitSilently = errors.New("exit 1")

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if !errors.Is(err, errExitSilently) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}
