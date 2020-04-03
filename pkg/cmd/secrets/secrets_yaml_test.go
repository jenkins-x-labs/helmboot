package secrets_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-yaml/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/secrets"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakejxfactory"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	expectedAdminUser     = "someuser"
	expectedAdminPassword = "dummypwd"
	expectedHmacToken     = "TODO"
	expectedPipelineUser  = "somepipelineuser"
	expectedPipelineToken = "dummmytoken"
	expectedPipelineEmail = "me@foo.com"

	expectedYaml = `secrets:
  adminUser:
    password: dummypwd 
    username: admin
  hmacToken:  TODO
  pipelineUser:
    email: me@foo.com
    token: dummmytoken 
    username: somepipelineuser 
`
)

var (
	testSecretData = map[string][]byte{
		"adminUser.username":    []byte(expectedAdminUser),
		"adminUser.password":    []byte(expectedAdminPassword),
		"hmacToken":             []byte(expectedHmacToken),
		"pipelineUser.username": []byte(expectedPipelineUser),
		"pipelineUser.token":    []byte(expectedPipelineToken),
		"pipelineUser.email":    []byte(expectedPipelineEmail),
	}
)

func TestSecretsYAMLWithEntries(t *testing.T) {
	outFile, err := ioutil.TempFile("", "test-helmboot-secret-yaml-")
	require.NoError(t, err, "failed to create a temporary dir")
	outFileName := outFile.Name()

	_, yo := secrets.NewCmdYAML()

	ns := "jx"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretmgr.LocalSecret,
			Namespace: ns,
			Labels:    map[string]string{},
		},
		Data: testSecretData,
	}
	k8sObjects := []runtime.Object{secret}
	f := fakejxfactory.NewFakeFactoryWithObjects(k8sObjects, nil, ns)
	yo.JXFactory = f
	yo.OutFile = outFileName
	err = yo.Run()
	require.NoErrorf(t, err, "should not have failed to create YAML")

	assertGeneratedYAMLFileIsValid(t, outFileName)
}

func assertGeneratedYAMLFileIsValid(t *testing.T, outFileName string) {
	assert.FileExists(t, outFileName, "did not generate output YAML file")
	data, err := ioutil.ReadFile(outFileName)
	require.NoErrorf(t, err, "failed to load generated YAML")
	got := string(data)

	t.Logf("generated YAML:\n")
	t.Logf("\n%s\n", got)

	m := map[string]interface{}{}
	err = yaml.Unmarshal(data, &m)
	require.NoErrorf(t, err, "failed to unmarshal generated YAML")

	secrets := m["secrets"]
	require.NotNil(t, secrets, "could not find secrets")
	sm, ok := secrets.(map[interface{}]interface{})
	require.True(t, ok, "secrets value is not a map %#v", secrets)
	require.NotNil(t, sm, "secrets value is nil")

	sm2 := convertMaps(sm)
	for k, v := range testSecretData {
		want := string(v)
		got := util.GetMapValueAsStringViaPath(sm2, k)

		t.Logf("key %s = '%s'", k, got)
		assert.Equal(t, want, got, "for %s", k)
	}
}

func TestSecretsYAMLWithYAML(t *testing.T) {
	outFile, err := ioutil.TempFile("", "test-helmboot-secret-yaml-")
	require.NoError(t, err, "failed to create a temporary dir")
	outFileName := outFile.Name()

	_, yo := secrets.NewCmdYAML()

	ns := "jx"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretmgr.LocalSecret,
			Namespace: ns,
			Labels:    map[string]string{},
		},
		Data: map[string][]byte{
			secretmgr.LocalSecretKey: []byte(expectedYaml),
		},
	}
	k8sObjects := []runtime.Object{secret}
	f := fakejxfactory.NewFakeFactoryWithObjects(k8sObjects, nil, ns)
	yo.JXFactory = f
	yo.OutFile = outFileName
	err = yo.Run()
	require.NoErrorf(t, err, "should not have failed to create YAML")

	assert.FileExists(t, outFileName, "did not generate output YAML file")
	data, err := ioutil.ReadFile(outFileName)
	require.NoErrorf(t, err, "failed to load generated YAML")
	got := string(data)

	t.Logf("generated YAML:\n")
	t.Logf("\n%s\n", got)

	if diff := cmp.Diff(strings.TrimSpace(got), strings.TrimSpace(expectedYaml)); diff != "" {
		t.Errorf("Unexpected generated YAML")
		t.Log(diff)
	}
}

func TestSecretsYAMLFromFile(t *testing.T) {
	outFile, err := ioutil.TempFile("", "test-helmboot-secret-yaml-")
	require.NoError(t, err, "failed to create a temporary dir")
	outFileName := outFile.Name()

	_, yo := secrets.NewCmdYAML()

	yo.SecretFile = filepath.Join("test_data", "sample_secrets.txt")

	ns := "jx"
	f := fakejxfactory.NewFakeFactoryWithObjects(nil, nil, ns)
	yo.JXFactory = f
	yo.OutFile = outFileName
	err = yo.Run()
	require.NoErrorf(t, err, "should not have failed to create YAML")

	assertGeneratedYAMLFileIsValid(t, outFileName)
}

func convertMaps(sm map[interface{}]interface{}) map[string]interface{} {
	sm2 := map[string]interface{}{}
	for k, v := range sm {
		text, ok := k.(string)
		if ok {
			m, ok := v.(map[interface{}]interface{})
			if ok {
				v = convertMaps(m)
			}
			sm2[text] = v
		}
	}
	return sm2
}
