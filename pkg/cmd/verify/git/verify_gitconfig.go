package git

import (
	"context"
	"fmt"

	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/envfactory"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	verifyGitConfigLong = templates.LongDesc(`
		Verifies the git configuration
`)

	verifyGitConfigExample = templates.Examples(`
		# verifies the git config and token
		%s step verify git config
	`)
)

// VerifyGitTokenOptions the options for viewing running PRs
type VerifyGitTokenOptions struct {
	envfactory.EnvFactory
}

// NewCmdVerifyGitToken creates a command object for the command
func NewCmdVerifyGitToken() (*cobra.Command, *VerifyGitTokenOptions) {
	o := &VerifyGitTokenOptions{}

	cmd := &cobra.Command{
		Use:     "git config",
		Short:   "Verifies the git configuration",
		Long:    verifyGitConfigLong,
		Example: fmt.Sprintf(verifyGitConfigExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	o.EnvFactory.AddFlags(cmd)

	return cmd, o
}

// Run implements the command
func (o *VerifyGitTokenOptions) Run() error {
	scmClient, _, err := o.EnvFactory.CreateScmClient("https://github.com", "", "github")
	if err != nil {
		return err
	}
	ctx := context.Background()

	user, _, err := scmClient.Users.Find(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to lookup the current user name")
	}
	log.Logger().Infof("current git user is %s", user.Login)
	return nil
}
