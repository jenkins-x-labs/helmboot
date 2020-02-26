package secrets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/factory"
	"github.com/jenkins-x/jx/pkg/apps"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/secreturl"
	"github.com/jenkins-x/jx/pkg/surveyutils"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	/* #nosec */
	defaultSecretSchemaTemplate = `{
  "$id": "https:/jenkins-x.io/tests/descriptionAndTitle.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "secretsSchema.yaml",
  "type": "object",
  "properties": {
    "adminUser": {
      "type": "object",
      "required": [
        "username",
        "password"
      ],
      "properties": {
        "username": {
          "type": "string",
          "title": "Jenkins X Admin Username",
          "description": "The Admin Username will be used by all services installed by Jenkins X",
          "default": "admin"
        },
        "password": {
          "type": "string",
          "format": "password",
          "title": "Jenkins X Admin Password",
          "description": "The Admin Password will be used by all services installed by Jenkins X"
        }
      }
    },
    "pipelineUser": {
      "type": "object",
      "required": [
        "username",
        "email",
        "token"
      ],
      "properties": {
        "username": {
          "type": "string",
          "title": "Pipeline bot Git username",
          "description": "The Git user that will perform git operations inside a pipeline. It should be a user within the Git organisation/owner where environment repositories will live. This is normally a bot."
        },
        "email": {
          "type": "string",
          "title": "Pipeline bot Git email address",
          "description": "The email address of the Git user that will perform git operations inside a pipeline."
        },
{{- if eq .GitKind "github" }}
        "token": {
          "type": "string",
          "format": "token",
          "title": "Pipeline bot Git token",
          "description": "A token for the Git user that will perform git operations inside a pipeline. This includes environment repository creation, and so this token should have full repository permissions. To create a token go to {{ .GitServer }}/settings/tokens/new?scopes=repo,read:user,read:org,user:email,write:repo_hook,delete_repo then enter a name, click Generate token, and copy and paste the token into this prompt.",
          "minLength": 40,
          "maxLength": 40,
          "pattern": "^[0-9a-f]{40}$"
        }
{{- else if eq .GitKind "bitbucketserver" }}
        "token": {
          "type": "string",
          "format": "token",
          "title": "Pipeline bot Git token",
          "description": "A token for the Git user that will perform git operations inside a pipeline. This includes environment repository creation, and so this token should have full repository permissions. To create a token go to {{ .GitServer }}/plugins/servlet/access-tokens/manage then enter a name, click Generate token, and copy and paste the token into this prompt.",
          "minLength": 8,
          "maxLength": 50
        }
{{- else if eq .GitKind "gitlab" }}
        "token": {
          "type": "string",
          "format": "token",
          "title": "Pipeline bot Git token",
          "description": "A token for the Git user that will perform git operations inside a pipeline. This includes environment repository creation, and so this token should have full repository permissions. To create a token go to {{ .GitServer }}/profile/personal_access_tokens then enter a name, click Generate token, and copy and paste the token into this prompt.",
          "minLength": 8,
          "maxLength": 50
        }
{{- else }}
        "token": {
          "type": "string",
          "format": "token",
          "title": "Pipeline bot Git token",
          "description": "A token for the Git user that will perform git operations inside a pipeline. This includes environment repository creation, and so this token should have full repository permissions. To create a token go to {{ .GitServer }}/settings/tokens/new?scopes=repo,read:user,read:org,user:email,write:repo_hook,delete_repo then enter a name, click Generate token, and copy and paste the token into this prompt.",
          "minLength": 8,
          "maxLength": 50
        }
{{- end }}
      }
    },
    "hmacToken": {
      "type": "string",
      "format": "token",
      "title": "HMAC token, used to validate incoming webhooks. Press enter to use the generated token",
      "description": "The HMAC token is used by the Git Provider to create a hash signature for each webhook, and by Jenkins X to validate that the signature is from a trusted source. It's normally best to have Jenkins X generate a token for you if you don't already have one. You'll need to save it and use it with all the webhooks configured in your git provider for Jenkins X. For more detail see: https://en.wikipedia.org/wiki/HMAC",
      "default": "<generated:hmac>"
    }
  }
}
`
)

var (
	editLong = templates.LongDesc(`
		Edits all or the missing secrets and stores them in the underlying Secret Manager
`)

	editExample = templates.Examples(`
		# edit the secrets
		%s secrets edit
	`)
)

// EditOptions the options for viewing running PRs
type EditOptions struct {
	factory.KindResolver
	SchemaFile    string
	IOFileHandles *util.IOFileHandles
	AskExisting   bool
	BatchMode     bool
	Verbose       bool
}

// NewCmdEdit creates a command object for the command
func NewCmdEdit() (*cobra.Command, *EditOptions) {
	o := &EditOptions{}

	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "Edits the secrets",
		Long:    editLong,
		Example: fmt.Sprintf(editExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&o.SchemaFile, "schema-file", "s", "secrets/secrets.schema.json", "the JSON schema file to use to validate the secrets")
	cmd.Flags().BoolVarP(&o.AskExisting, "all", "a", false, "if enabled ask for confirmation on all secret values. Otherwise just prompt for missing values only")
	cmd.Flags().BoolVarP(&o.Verbose, "verbose", "v", false, "enables verbose logging")
	cmd.Flags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Runs in batch mode without prompting for user input")

	AddKindResolverFlags(cmd, &o.KindResolver)
	return cmd, o
}

// Run implements the command
func (o *EditOptions) Run() error {
	sm, err := o.CreateSecretManager()
	if err != nil {
		return err
	}

	secretsYaml := ""
	err = sm.UpsertSecrets(func(currentYaml string) (string, error) {
		secretsYaml = currentYaml
		return currentYaml, nil
	}, secretmgr.DefaultSecretsYaml)
	if err != nil {
		return errors.Wrapf(err, "failed to load the Secrets YAML from secret manager %s", sm.String())
	}

	updatedYaml, err := o.editSecretsYaml(secretsYaml)
	if err != nil {
		return err
	}

	err = sm.UpsertSecrets(func(string) (string, error) {
		return updatedYaml, nil
	}, secretmgr.DefaultSecretsYaml)
	if err != nil {
		return errors.Wrapf(err, "failed to update the Secrets YAML from secret manager %s", sm.String())
	}
	log.Logger().Infof("edited the Secrets in %s", sm.String())
	return nil
}

func (o *EditOptions) editSecretsYaml(secretsYaml string) (string, error) {
	requirements := o.Requirements
	if requirements == nil {
		return secretsYaml, fmt.Errorf("no jx-requirements.yml configured")
	}

	err := o.generateSchemaFile(requirements)
	if err != nil {
		return secretsYaml, err
	}

	secretClient := NewMemoryClient()
	if strings.TrimSpace(secretsYaml) != "" {
		err = yaml.Unmarshal([]byte(secretsYaml), &secretClient.Data)
		if err != nil {
			return secretsYaml, errors.Wrap(err, "failed to unmarshal YAML")
		}
	}
	existing := map[string]interface{}{}
	existingSecrets := secretClient.Data["secrets"]
	existingSecretsMap, ok := existingSecrets.(map[string]interface{})
	if ok {
		for k, v := range existingSecretsMap {
			if v != nil && v != "" {
				existing[k] = v
			}
		}
	}
	removeMapEmptyValues(existing)

	nonPasswordYAML, err := o.populateValues(secretClient, existing)
	if err != nil {
		return secretsYaml, err
	}

	if strings.TrimSpace(nonPasswordYAML) != "" {
		nonSecrets := map[string]interface{}{}
		err = yaml.Unmarshal([]byte(nonPasswordYAML), &nonSecrets)
		if err != nil {
			return secretsYaml, errors.Wrapf(err, "failed to unmarshal non passwords YAML: %s", nonPasswordYAML)
		}

		overrides := map[string]interface{}{
			"secrets": nonSecrets,
		}

		util.CombineMapTrees(overrides, secretClient.Data)
		secretClient.Data = overrides
	}

	updatedYaml, err := secretClient.ToYAML()
	if err != nil {
		return updatedYaml, err
	}
	return updatedYaml, nil
}

// removeMapEmptyValues recursively removes all empty string or nil entries
func removeMapEmptyValues(m map[string]interface{}) {
	for k, v := range m {
		if v == nil || v == "" {
			delete(m, k)
		}
		childMap, ok := v.(map[string]interface{})
		if ok {
			removeMapEmptyValues(childMap)
		}
	}
}

func (o *EditOptions) generateSchemaFile(requirements *config.RequirementsConfig) error {
	templateFile := strings.TrimSuffix(o.SchemaFile, ".schema.json") + ".tmpl.schema.json"
	schemaExists := false
	for _, f := range []string{o.SchemaFile, templateFile} {
		exists, err := util.FileExists(f)
		if err != nil {
			return errors.Wrap(err, "failed to check file exists")
		}
		if exists {
			schemaExists = true
			break
		}
	}
	if !schemaExists {
		// lets download the default schema template from the build pack
		err := o.findDefaultSchemaTemplate(templateFile)
		if err != nil {
			return err
		}
	}

	err := surveyutils.TemplateSchemaFile(o.SchemaFile, requirements)
	if err != nil {
		return errors.Wrapf(err, "failed to generate %s from template", o.SchemaFile)
	}
	return nil
}

// findDefaultSchemaTemplate TODO lets use the build pack to generate the schema file
// allowing different Apps to share or contribute to different schema fragments
func (o *EditOptions) findDefaultSchemaTemplate(templateFileName string) error {
	dir := filepath.Dir(templateFileName)
	err := os.MkdirAll(dir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to create directory for secrets schema file")
	}

	err = ioutil.WriteFile(templateFileName, []byte(defaultSecretSchemaTemplate), util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to save default secrets schema template")
	}
	return nil
}

// populateValues builds the clients and settings from the team needed to run apps.ProcessValues, and then copies the answer
// to the location requested by the command
func (o *EditOptions) populateValues(secretURLClient secreturl.Client, existing map[string]interface{}) (string, error) {
	schema, err := ioutil.ReadFile(o.SchemaFile)
	if err != nil {
		return "", errors.Wrapf(err, "reading schema %s", o.SchemaFile)
	}
	vaultScheme := "vault:"
	vaultBasePath := "/secrets"

	handles := common.GetIOFileHandles(o.IOFileHandles)
	values, err := apps.GenerateQuestions(schema, o.BatchMode, o.AskExisting, vaultBasePath, secretURLClient, existing, vaultScheme, handles)
	if err != nil {
		return "", errors.Wrapf(err, "asking questions for schema")
	}
	return string(values), nil
}
