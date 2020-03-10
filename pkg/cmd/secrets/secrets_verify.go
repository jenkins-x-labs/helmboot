package secrets

import (
	"fmt"

	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/factory"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

var (
	verifyLong = templates.LongDesc(`
		Verifies the secrets are populated correctly
`)

	verifyExample = templates.Examples(`
		# verifies the secrets are setup correctly
		%s secrets verify
	`)
)

// VerifyOptions the options for viewing running PRs
type VerifyOptions struct {
	factory.KindResolver
	File string
}

// NewCmdVerify creates a command object for the command
func NewCmdVerify() (*cobra.Command, *VerifyOptions) {
	o := &VerifyOptions{}

	cmd := &cobra.Command{
		Use:     "verify",
		Short:   "Verifies the secrets are populated correctly",
		Long:    verifyLong,
		Example: fmt.Sprintf(verifyExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	AddKindResolverFlags(cmd, &o.KindResolver)
	return cmd, o
}

// Run implements the command
func (o *VerifyOptions) Run() error {
	err := o.VerifySecrets()
	if err != nil {
		return err
	}
	log.Logger().Infof("secrets are valid")
	return nil
}
