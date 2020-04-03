package factory

import (
	"fmt"
	"strings"

	"github.com/jenkins-x-labs/helmboot/pkg/reqhelpers"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KindResolver provides a simple way to resolve what kind of Secret Manager to use
type KindResolver struct {
	Factory jxfactory.Factory
	Kind    string
	Dir     string
	GitURL  string

	// outputs which can be useful
	DevEnvironment *v1.Environment
	Requirements   *config.RequirementsConfig
}

// CreateSecretManager detects from the current cluster which kind of SecretManager to use and then creates it
func (r *KindResolver) CreateSecretManager() (secretmgr.SecretManager, error) {
	if r.Factory == nil {
		r.Factory = jxfactory.NewFactory()
	}
	// lets try find the requirements from the cluster or locally
	requirements, ns, err := r.resolveRequirements()
	if err != nil {
		return nil, err
	}
	r.Requirements = requirements

	if requirements == nil {
		return nil, fmt.Errorf("failed to resolve the jx-requirements.yml from the file system or the 'dev' Environment in namespace %s", ns)
	}
	if r.Kind == "" {
		var err error
		r.Kind, err = r.resolveKind(requirements)
		if err != nil {
			return nil, err
		}

		// if we can't find one default to local Secrets
		if r.Kind == "" {
			r.Kind = secretmgr.KindLocal
		}
	}
	return NewSecretManager(r.Kind, r.Factory, requirements)
}

// VerifySecrets verifies that the secrets are valid
func (r *KindResolver) VerifySecrets() error {
	secretsYAML := ""
	sm, err := r.CreateSecretManager()
	if err != nil {
		return err
	}

	cb := func(currentYAML string) (string, error) {
		secretsYAML = currentYAML
		return currentYAML, nil
	}
	err = sm.UpsertSecrets(cb, secretmgr.DefaultSecretsYaml)
	if err != nil {
		return errors.Wrapf(err, "failed to load Secrets YAML from secret manager %s", sm.String())
	}

	secretsYAML = strings.TrimSpace(secretsYAML)
	if secretsYAML == "" {
		return errors.Errorf("empty secrets YAML")
	}
	return secretmgr.VerifyBootSecrets(secretsYAML)
}

func (r *KindResolver) resolveKind(requirements *config.RequirementsConfig) (string, error) {
	if requirements.SecretStorage == config.SecretStorageTypeVault {
		return secretmgr.KindVault, nil
	}
	if requirements.Cluster.Provider == cloud.GKE {
		// lets check if we have a Local secret otherwise default to Google

		kubeClient, ns, err := r.Factory.CreateKubeClient()
		if err != nil {
			return "", errors.Wrap(err, "failed to create Kubernetes client")
		}
		name := secretmgr.LocalSecret
		_, err = kubeClient.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return "", errors.Wrapf(err, "failed to get Secret %s in namespace %s", name, ns)
			}
			// no secret so lets assume gcloud
			return secretmgr.KindGoogleSecretManager, nil
		}
	}
	return secretmgr.KindLocal, nil
}

func (r *KindResolver) resolveRequirements() (*config.RequirementsConfig, string, error) {
	jxClient, ns, err := r.Factory.CreateJXClient()
	if err != nil {
		return nil, ns, errors.Wrap(err, "failed to create JX Client")
	}

	dev, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, ns, errors.Wrap(err, "failed to find the 'dev' Environment resource")
	}
	r.DevEnvironment = dev
	if r.Requirements != nil {
		return r.Requirements, ns, nil
	}
	if dev != nil {
		requirements, err := config.GetRequirementsConfigFromTeamSettings(&dev.Spec.TeamSettings)
		if err != nil {
			return nil, ns, errors.Wrapf(err, "failed to unmarshal requirements from 'dev' Environment in namespace %s", ns)
		}
		if requirements != nil {
			return requirements, ns, nil
		}
	}
	if r.GitURL != "" {
		requirements, err := reqhelpers.GetRequirementsFromGit(r.GitURL)
		return requirements, ns, err
	}

	requirements, _, err := config.LoadRequirementsConfig(r.Dir)
	if err != nil {
		return requirements, ns, errors.Wrapf(err, "failed to requirements YAML file from %s", r.Dir)
	}
	return requirements, ns, nil
}
