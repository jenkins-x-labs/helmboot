package secrets

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var (
	yamlLong = templates.LongDesc(`
		Edits all or the missing secrets and stores them in the underlying Secret Manager
`)

	yamlExample = templates.Examples(`
		# edit the secrets
		%s secrets edit
	`)
)

// YAMLOptions the options for viewing running PRs
type YAMLOptions struct {
	JXFactory  jxfactory.Factory
	SecretName string
	OutFile    string
	BatchMode  bool
	Verbose    bool
}

// NewCmdYAML creates a command object for the command
func NewCmdYAML() (*cobra.Command, *YAMLOptions) {
	o := &YAMLOptions{}

	cmd := &cobra.Command{
		Use:     "yaml",
		Short:   "Generates the YAML file from a Kuberentes Secret",
		Long:    yamlLong,
		Example: fmt.Sprintf(yamlExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&o.OutFile, "out", "o", "", "The output YAML file to generate")
	cmd.Flags().BoolVarP(&o.Verbose, "verbose", "v", false, "enables verbose logging")
	cmd.Flags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Runs in batch mode without prompting for user input")
	return cmd, o
}

// Run implements the command
func (o *YAMLOptions) Run() error {
	if o.JXFactory == nil {
		o.JXFactory = jxfactory.NewFactory()
	}

	kubeClient, ns, err := o.JXFactory.CreateKubeClient()
	if err != nil {
		return err
	}

	secretName := o.SecretName
	if secretName == "" {
		secretName = os.Getenv("JXL_SECRET_NAME")
	}
	if secretName == "" {
		secretName = secretmgr.LocalSecret
	}

	secret, err := kubeClient.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("could not read Secret %s in namespace %s", secretName, ns)
		}
		return errors.Wrapf(err, "failed to read Secret %s in namespace %s", secretName, ns)
	}
	data := secret.Data
	if len(data) == 0 {
		return fmt.Errorf("no data for Secret %s in namespace %s", secretName, ns)
	}

	if o.OutFile == "" {
		o.OutFile = os.Getenv("JX_SECRETS_YAML")
	}
	if o.OutFile == "" {
		return util.MissingOption("out")
	}
	return generateSecretsYAML(o.OutFile, data)
}

func generateSecretsYAML(fileName string, secretData map[string][]byte) error {
	var err error
	data := secretData[secretmgr.LocalSecretKey]
	if len(data) == 0 {
		secrets := map[string]interface{}{}
		for k, v := range secretData {
			util.SetMapValueViaPath(secrets, k, string(v))
		}
		values := map[string]interface{}{
			"secrets": secrets,
		}

		data, err = yaml.Marshal(values)
		if err != nil {
			return errors.Wrap(err, "failed to marshal data to YAML")
		}
	}

	err = ioutil.WriteFile(fileName, data, util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", fileName)
	}
	log.Logger().Infof("generated secrets file %s", util.ColorInfo(fileName))
	return nil
}
