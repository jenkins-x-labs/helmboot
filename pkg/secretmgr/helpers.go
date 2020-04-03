package secretmgr

import (
	"strings"

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

// ToSecretsYAML converts the data to secrets YAML
func ToSecretsYAML(values map[string]interface{}) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	secrets := map[string]interface{}{
		"secrets": values,
	}
	data, err := yaml.Marshal(secrets)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal data to YAML")
	}
	return string(data), nil
}

// RemoveMapEmptyValues recursively removes all empty string or nil entries
func RemoveMapEmptyValues(m map[string]interface{}) {
	for k, v := range m {
		if v == nil || v == "" {
			delete(m, k)
		}
		childMap, ok := v.(map[string]interface{})
		if ok {
			RemoveMapEmptyValues(childMap)
		}
	}
}

// UnmarshalSecretsYAML unmarshals the given Secrets YAML
func UnmarshalSecretsYAML(secretsYaml string) (map[string]interface{}, error) {
	data := map[string]interface{}{}

	if strings.TrimSpace(secretsYaml) != "" {
		err := yaml.Unmarshal([]byte(secretsYaml), &data)
		if err != nil {
			return data, errors.Wrap(err, "failed to unmarshal YAML")
		}
	}
	existing := map[string]interface{}{}
	existingSecrets := data["secrets"]
	existingSecretsMap, ok := existingSecrets.(map[string]interface{})
	if ok {
		for k, v := range existingSecretsMap {
			if v != nil && v != "" {
				existing[k] = v
			}
		}
	}
	RemoveMapEmptyValues(existing)
	return existing, nil
}
