package cmd

import (
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/create"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/run"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/secrets"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/show"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/step"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/upgrade"
	"github.com/jenkins-x-labs/helmboot/pkg/common"
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
	cmd.AddCommand(run.NewCmdRun())
	cmd.AddCommand(secrets.NewCmdSecrets())
	cmd.AddCommand(step.NewCmdStep())

	cmd.AddCommand(common.SplitCommand(create.NewCmdCreate()))
	cmd.AddCommand(common.SplitCommand(upgrade.NewCmdUpgrade()))
	cmd.AddCommand(common.SplitCommand(show.NewCmdShow()))
	return cmd
}
