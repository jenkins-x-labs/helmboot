package testhelpers

import (
	"fmt"
	"testing"

	"github.com/go-yaml/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertYamlEqual validates YAML without worrying about ordering of keys
func AssertYamlEqual(t *testing.T, expected string, actual string, message string, args ...interface{}) {
	expectedMap := map[interface{}]interface{}{}
	actualMap := map[interface{}]interface{}{}

	reason := fmt.Sprintf(message, args...)

	err := yaml.Unmarshal([]byte(expected), &expectedMap)
	require.NoError(t, err, "failed to unmarshal expected yaml: %s for %s", expected, reason)

	err = yaml.Unmarshal([]byte(actual), &actualMap)
	require.NoError(t, err, "failed to unmarshal actual yaml: %s for %s", actual, reason)

	assert.Equal(t, expectedMap, actualMap, "parsed YAML contents not equal for %s", reason)
}
