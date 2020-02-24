package step

import (
	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// NewCmdStep creates the new command
func NewCmdStep() *cobra.Command {
	command := &cobra.Command{
		Use:     "step",
		Short:   "defines some pipeline steps for use inside boot",
		Aliases: []string{"secret"},
		Run: func(command *cobra.Command, args []string) {
			err := command.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	command.AddCommand(common.SplitCommand(NewCmdStatus()))
	return command
}
