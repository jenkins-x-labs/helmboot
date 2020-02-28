package create_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/jenkins-x-labs/helmboot/pkg/cmd/create"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakegit"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakejxfactory"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	//t.Parallel()

	os.Setenv("JX_SECRETS_YAML", "test_data/secrets.yaml")

	type testCase struct {
		Name string
		Args []string
	}
	testCases := []testCase{
		{
			Name: "defaultcluster",
		},
	}

	for _, tc := range testCases {
		t.Logf("running test: %s", tc.Name)
		_, co := create.NewCmdCreate()
		co.BatchMode = true
		co.Gitter = fakegit.NewGitFakeClone()
		co.DisableVerifyPackages = true
		outFile, err := ioutil.TempFile("", "")
		require.NoError(t, err, "failed to create tempo file")
		outFileName := outFile.Name()
		args := []string{"--provider", "kubernetes", "--cluster", tc.Name, "--git-server", "https://fake.com", "--git-kind", "fake", "--env-git-owner", "jstrachan", "--out", outFileName, "--env-git-public", "--git-public"}
		args = append(args, tc.Args...)
		co.Args = args
		co.JXFactory = fakejxfactory.NewFakeFactory()

		err = co.Run()
		require.NoError(t, err, "failed to create repository for test %s", tc.Name)

		// now lets assert we created a new repository
		ctx := context.Background()
		repoName := fmt.Sprintf("environment-%s-dev", tc.Name)
		fullName := fmt.Sprintf("jstrachan/%s", repoName)

		repo, _, err := co.EnvFactory.ScmClient.Repositories.Find(ctx, fullName)
		require.NoError(t, err, "failed to find repository %s", fullName)
		assert.NotNil(t, repo, "nil repository %s", fullName)
		assert.Equal(t, fullName, repo.FullName, "repo.FullName for %s", tc.Name)
		assert.Equal(t, repoName, repo.Name, "repo.FullName for %s", tc.Name)

		t.Logf("test %s created dir %s\n", tc.Name, co.OutDir)

		apps, appFileName, err := config.LoadAppConfig(co.OutDir)
		require.NoError(t, err, "failed to load the apps configuration in dir %s for test %s", co.OutDir, tc.Name)
		appMessage := fmt.Sprintf("test %s for file %s", tc.Name, appFileName)

		AssertHasApp(t, apps, "jenkins-x/lighthouse", appMessage)

		assert.FileExists(t, outFileName, "did not generate the Git URL file")
		data, err := ioutil.ReadFile(outFileName)
		require.NoError(t, err, "failed to load file %s", outFileName)
		text := strings.TrimSpace(string(data))
		expectedGitURL := fmt.Sprintf("https://fake.com/jstrachan/environment-%s-dev.git", tc.Name)
		assert.Equal(t, expectedGitURL, text, "output Git URL")

		requirements, _, err := config.LoadRequirementsConfig(co.OutDir)
		require.NoError(t, err, "failed to load requirements from %s", co.OutDir)
		assert.Equal(t, true, requirements.Cluster.EnvironmentGitPublic, "requirements.Cluster.EnvironmentGitPublic")
		assert.Equal(t, true, requirements.Cluster.GitPublic, "requirements.Cluster.GitPublic")
	}
}

func AssertHasApp(t *testing.T, appConfig *config.AppConfig, appName string, message string) {
	found := false
	names := []string{}
	for _, app := range appConfig.Apps {
		names = append(names, app.Name)
		if app.Name == appName {
			t.Logf("has app %s for %s", appName, message)
			found = true
		}
	}
	if !found {
		assert.Fail(t, fmt.Sprintf("does not have the app %s for %s. Current apps are: %s", appName, message, strings.Join(names, ", ")))
	}

}
