package secrets_test

import (
	"io/ioutil"
	"testing"

	"github.com/jenkins-x-labs/helmboot/pkg/cmd/secrets"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakejxfactory"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	f := fakejxfactory.NewFakeFactory()
	eo.Factory = f
	io.Factory = f

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
}
