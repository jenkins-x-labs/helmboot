package cmd

import (
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/create"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/run"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/show"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/upgrade"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// HelmBoot creates the new command
func HelmBoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "helmboot",
		Short: "boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps",
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	cmd.AddCommand(run.NewHelmBootRun())

	cmd.AddCommand(SplitCommand(create.NewCmdCreate()))
	cmd.AddCommand(SplitCommand(upgrade.NewCmdUpgrade()))
	cmd.AddCommand(SplitCommand(show.NewCmdShow()))
	return cmd
}

// SplitCommand helper command to ignore the options object
func SplitCommand(cmd *cobra.Command, options interface{}) *cobra.Command {
	return cmd
}
