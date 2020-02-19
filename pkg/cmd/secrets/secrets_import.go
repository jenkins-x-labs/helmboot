package secrets

import (
	"io/ioutil"

	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/factory"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	importLong = templates.LongDesc(`
		Imports the secrets from the local file system to where they are stored (cloud secret manager / vault / kubernetes Secret)
`)

	importExample = templates.Examples(`
		# imports the secrets
		helmboot secrets import -f /tmp/mysecrets.yaml
	`)
)

// ImportOptions the options for viewing running PRs
type ImportOptions struct {
	Factory jxfactory.Factory
	File    string
}

// NewCmdImport creates a command object for the "create" command
func NewCmdImport() (*cobra.Command, *ImportOptions) {
	o := &ImportOptions{}

	cmd := &cobra.Command{
		Use:     "import",
		Short:   "imports the secrets from the local file system",
		Long:    importLong,
		Example: importExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&o.File, "file", "f", "", "the file to load the Secrets YAML from")
	return cmd, o
}

// Run implements the command
func (o *ImportOptions) Run() error {
	fileName := o.File
	if fileName == "" {
		return util.MissingOption("file")
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to load file %s", fileName)
	}

	secretsYAML := string(data)
	sm, err := factory.CreateSecretManager(o.Factory)
	if err != nil {
		return err
	}

	cb := func(secretsYaml string) (string, error) {
		return secretsYAML, nil
	}
	err = sm.UpsertSecrets(cb, secretmgr.DefaultSecretsYaml)
	if err != nil {
		return errors.Wrapf(err, "failed to import Secrets YAML from secret manager %s", sm.String())
	}
	log.Logger().Infof("imported Secrets to %s from file: %s", sm.String(), util.ColorInfo(fileName))
	return nil
}
