package main

import (
	"github.com/spf13/cobra"

	"github.com/2005Hari/campuscloud/internal/config"
	"github.com/2005Hari/campuscloud/internal/dockercli"
)

// app bundles the configuration and Docker client every subcommand needs.
// It is populated once in the root command's PersistentPreRunE.
type app struct {
	cfg    *config.Config
	client *dockercli.Client
}

var (
	configPath string
	envPath    string
)

func newRootCmd() *cobra.Command {
	a := &app{}

	root := &cobra.Command{
		Use:   "campuscloud",
		Short: "Automated deployment, monitoring, backup, and recovery for the CampusCloud private cloud",
		Long: `CampusCloud is a Go CLI that automates deployment, configuration,
monitoring, backup, and recovery of a containerized Nextcloud + MariaDB
private-cloud environment for colleges and student teams.`,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath, envPath)
			if err != nil {
				return err
			}
			a.cfg = cfg
			a.client = dockercli.NewDefault(cfg.Compose.File, cfg.Project.Name).WithEnvFile(envPath)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&configPath, "config", "config/config.yaml", "path to config.yaml")
	root.PersistentFlags().StringVar(&envPath, "env", ".env", "path to .env file")

	root.AddCommand(
		newDoctorCmd(a),
		newDeployCmd(a),
		newStartCmd(a),
		newStopCmd(a),
		newRestartCmd(a),
		newStatusCmd(a),
		newContainersCmd(a),
		newHealthCmd(a),
		newLogsCmd(a),
		newBackupCmd(a),
		newRestoreCmd(a),
		newConfigCmd(a),
		newVersionCmd(),
	)

	return root
}
