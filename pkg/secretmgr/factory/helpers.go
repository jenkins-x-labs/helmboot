package factory

import (
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x/jx/pkg/jxfactory"
)

// CreateSecretManager detects from the current cluster which kind of SecretManager to use and then creates it
func CreateSecretManager(f jxfactory.Factory) (secretmgr.SecretManager, error) {
	kind := secretmgr.KindLocal
	if f == nil {
		f = jxfactory.NewFactory()
	}

	// TODO how to detect google - find it in the requirements?
	return NewSecretManager(kind, f)
}
