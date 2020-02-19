package secrets

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/factory"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	exportLong = templates.LongDesc(`
		Exports the secrets from where they are stored (cloud secret manager / vault / kubernetes Secret) to the local file system
`)

	exportExample = templates.Examples(`
		# exports the secrets from where they are stored (cloud secret manager / vault / kubernetes Secret)
		helmboot secrets export -f /tmp/secrets/mysecrets.yaml
	`)
)

// ExportOptions the options for viewing running PRs
type ExportOptions struct {
	factory.KindResolver
	OutFile string
}

// NewCmdExport creates a command object for the "create" command
func NewCmdExport() (*cobra.Command, *ExportOptions) {
	o := &ExportOptions{}

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Exports the secrets to the local file system",
		Long:    exportLong,
		Example: exportExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&o.OutFile, "file", "f", "/tmp/secrets/helmboot/secrets.yaml", "the file to use to save the secrets to")

	AddKindResolverFlags(cmd, &o.KindResolver)
	return cmd, o
}

// AddKindResolverFlags adds the CLI arguments for specifying how to resolve the secret manager kind
func AddKindResolverFlags(cmd *cobra.Command, o *factory.KindResolver) {
	cmd.Flags().StringVarP(&o.Kind, "kind", "k", "", "the kind of Secret Manager you wish to use. If no value is supplied it is detected based on the jx-requirements.yml. Possible values are: "+strings.Join(secretmgr.KindValues, ", "))
	cmd.Flags().StringVarP(&o.Dir, "dir", "", ".", "the local directory used to find the jx-requirements.yml file if the cluster has not yet been booted")
}

// Run implements the command
func (o *ExportOptions) Run() error {
	fileName := o.OutFile
	if fileName == "" {
		return util.MissingOption("file")
	}
	dir := filepath.Dir(fileName)
	err := os.MkdirAll(dir, util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create parent directory %s", dir)
	}

	sm, err := o.CreateSecretManager()
	if err != nil {
		return err
	}

	secretsYAML := ""
	cb := func(secretsYaml string) (string, error) {
		secretsYAML = secretsYaml
		return secretsYaml, nil
	}

	err = sm.UpsertSecrets(cb, secretmgr.DefaultSecretsYaml)
	if err != nil {
		return errors.Wrapf(err, "failed to load Secrets YAML from secret manager %s", sm.String())
	}

	log.Logger().Infof("loaded Secrets from: %s", util.ColorInfo(sm.String()))

	err = ioutil.WriteFile(fileName, []byte(secretsYAML), util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save secrets file %s", fileName)
	}
	log.Logger().Infof("exported Secrets to file: %s", util.ColorInfo(fileName))
	return nil
}