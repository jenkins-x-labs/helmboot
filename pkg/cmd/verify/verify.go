package verify

import (
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/verify/git"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/verify/requirements"
	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// NewCmdVerify creates the new command
func NewCmdVerify() *cobra.Command {
	command := &cobra.Command{
		Use:   "verify",
		Short: "performs a number of verify steps",
		Run: func(command *cobra.Command, args []string) {
			err := command.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	command.AddCommand(common.SplitCommand(git.NewCmdVerifyGitToken()))
	command.AddCommand(common.SplitCommand(requirements.NewCmdRequirements()))
	return command
}
