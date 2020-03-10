package secrets

import (
	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// NewCmdSecrets creates the new command
func NewCmdSecrets() *cobra.Command {
	command := &cobra.Command{
		Use:     "secrets",
		Short:   "boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps. This is usually ran from inside the cluster",
		Aliases: []string{"secret"},
		Run: func(command *cobra.Command, args []string) {
			err := command.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	command.AddCommand(common.SplitCommand(NewCmdEdit()))
	command.AddCommand(common.SplitCommand(NewCmdExport()))
	command.AddCommand(common.SplitCommand(NewCmdImport()))
	command.AddCommand(common.SplitCommand(NewCmdVerify()))
	return command
}
