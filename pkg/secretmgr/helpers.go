package secretmgr

import (
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

var expectedSecretPaths = []string{
	"secrets.adminUser.username",
	"secrets.adminUser.password",
	"secrets.hmacToken",
	"secrets.pipelineUser.username",
	"secrets.pipelineUser.email",
	"secrets.pipelineUser.token",
}

// VerifyBootSecrets verifies the boot secrets
func VerifyBootSecrets(secretsYAML string) error {
	data := map[string]interface{}{}

	err := yaml.Unmarshal([]byte(secretsYAML), &data)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal secrets YAML")
	}

	// simple validation for now, using presence of a string value
	for _, path := range expectedSecretPaths {
		value := util.GetMapValueAsStringViaPath(data, path)
		if value == "" {
			return errors.Errorf("missing secret entry: %s", path)
		}
	}
	return nil
}
