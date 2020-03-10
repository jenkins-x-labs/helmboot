package secrets_test

import (
	"io/ioutil"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/jenkins-x-labs/helmboot/pkg/cmd/secrets"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakejxfactory"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	modifiedYaml = `secrets:
  adminUser:
    username: admin
    password: dummypwd 
  hmacToken:  TODO
  pipelineUser:
    username: someuser 
    token: dummmytoken 
    email: me@foo.com
`
)

func TestImportExportCommands(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "test-helmboot-secrets-")
	require.NoError(t, err, "failed to create a temporary file")

	_, eo := secrets.NewCmdExport()
	_, io := secrets.NewCmdImport()
	_, vo := secrets.NewCmdVerify()

	ns := "jx"
	devEnv := kube.CreateDefaultDevEnvironment(ns)
	devEnv.Namespace = ns
	req, err := config.GetRequirementsConfigFromTeamSettings(&devEnv.Spec.TeamSettings)
	if req == nil || err != nil {
		// lets populate some dummy requirements
		req = config.NewRequirementsConfig()
		reqBytes, err := yaml.Marshal(req)
		require.NoError(t, err, "there was a problem marshalling the requirements file to include it in the TeamSettings")
		devEnv.Spec.TeamSettings.BootRequirements = string(reqBytes)
	}
	jxObjects := []runtime.Object{devEnv}
	f := fakejxfactory.NewFakeFactoryWithObjects(nil, jxObjects, ns)
	eo.Factory = f
	io.Factory = f
	vo.Factory = f

	err = vo.Run()
	require.Errorf(t, err, "should have failed to verify secrets before they are imported")
	t.Logf("caught expected error when no secrets yet: %s", err.Error())

	fileName := tmpFile.Name()

	eo.OutFile = fileName
	err = eo.Run()
	require.NoError(t, err, "failed to export the secrets to %s", fileName)

	assert.FileExists(t, fileName, "exported file should exist")

	err = ioutil.WriteFile(fileName, []byte(modifiedYaml), util.DefaultFileWritePermissions)
	require.NoError(t, err, "failed to save file %s", fileName)

	io.File = fileName
	err = io.Run()
	require.NoError(t, err, "failed to import the secrets from %s", fileName)

	err = eo.Run()
	require.NoError(t, err, "failed to re-export the secrets to %s", fileName)

	data, err := ioutil.ReadFile(fileName)
	require.NoError(t, err, "failed to read the exported secrets file %s", fileName)
	actual := string(data)
	assert.Equal(t, modifiedYaml, actual, "the re-exported secrets YAML")

	err = vo.Run()
	require.NoError(t, err, "should not have failed to to verify secrets after they are imported")

}
