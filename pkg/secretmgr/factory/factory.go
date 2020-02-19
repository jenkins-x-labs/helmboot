package factory

import (
	"fmt"

	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/fake"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/local"
	"github.com/jenkins-x/jx/pkg/jxfactory"
)

// NewSecretManager creates a secret manager from a kind string
func NewSecretManager(kind string, f jxfactory.Factory) (secretmgr.SecretManager, error) {
	if f == nil {
		f = jxfactory.NewFactory()
	}
	switch kind {
	case secretmgr.KindLocal:
		return local.NewLocalSecretManager(f)
	case secretmgr.KindFake:
		return fake.NewFakeSecretManager(), nil
	default:
		return nil, fmt.Errorf("unknown secret manager kind: %s", kind)
	}
}
