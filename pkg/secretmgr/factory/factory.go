package factory

import (
	"fmt"

	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/fake"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/gsm"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/local"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/proxy"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jxfactory"
)

// NewSecretManager creates a secret manager from a kind string
func NewSecretManager(kind string, f jxfactory.Factory, requirements *config.RequirementsConfig) (secretmgr.SecretManager, error) {
	if f == nil {
		f = jxfactory.NewFactory()
	}
	switch kind {
	case secretmgr.KindGoogleSecretManager:
		// lets populate a local secret after importing/editing the google secret
		l, err := local.NewLocalSecretManager(f)
		if err != nil {
			return nil, err
		}
		g, err := gsm.NewGoogleSecretManager(requirements)
		if err != nil {
			return nil, err
		}
		return proxy.NewProxySecretManager(g, l), nil
	case secretmgr.KindLocal:
		return local.NewLocalSecretManager(f)
	case secretmgr.KindFake:
		return fake.NewFakeSecretManager(), nil
	default:
		return nil, fmt.Errorf("unknown secret manager kind: %s", kind)
	}
}
