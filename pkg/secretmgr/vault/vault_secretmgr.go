package vault

import (
	"fmt"

	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	vaultclient "github.com/jenkins-x-labs/helmboot/pkg/secretmgr/vault/client"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
)

// VaultSecretManager uses a Kubernetes Secret
type VaultSecretManager struct {
	Path   string
	client vaultclient.Client
}

// NewVaultSecretManagerFromJXFactory creates a secret manager from the jx factory
func NewVaultSecretManagerFromJXFactory(f jxfactory.Factory) (secretmgr.SecretManager, error) {
	clientFactory, err := vaultclient.NewFactoryFromJX(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vault client factory")
	}

	client, err := vaultclient.NewVaultClient(clientFactory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vault client")
	}
	return NewVaultSecretManager(client, "jx")
}

// NewVaultSecretManager creates a secret manager from the vault client
func NewVaultSecretManager(client vaultclient.Client, path string) (secretmgr.SecretManager, error) {
	return &VaultSecretManager{Path: path, client: client}, nil
}

func (v *VaultSecretManager) UpsertSecrets(callback secretmgr.SecretCallback, defaultYaml string) error {
	secretYaml, err := v.loadYaml()
	if err != nil {
		// lets assume its the first version
		log.Logger().Debugf("ignoring error %s", err.Error())
	}

	if secretYaml == "" {
		secretYaml = defaultYaml
	}

	updatedYaml, err := callback(secretYaml)
	if err != nil {
		return err
	}
	if updatedYaml != secretYaml {
		return v.updateSecretYaml(updatedYaml)
	}
	return nil
}

func (v *VaultSecretManager) Kind() string {
	return secretmgr.KindVault
}

func (v *VaultSecretManager) String() string {
	return fmt.Sprintf("Vault Secret Manager for vault %s", v.client.String())
}

func (v *VaultSecretManager) loadYaml() (string, error) {
	return vaultclient.ReadYaml(v.client, v.Path)
}

func (v *VaultSecretManager) updateSecretYaml(yaml string) error {
	return vaultclient.WriteYAML(v.client, v.Path, yaml)
}
