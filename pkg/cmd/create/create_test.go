package create_test

import (
	"context"
	"fmt"
	"io/ioutil"
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

	type testCase struct {
		Name string
		Args []string
	}
	testCases := []testCase{
		{
			Name: "remote",
			Args: []string{"--provider", "kubernetes", "--env-git-public", "--git-public", "--env-remote"},
		},
		{
			Name: "bucketrepo",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--repository", "bucketrepo"},
		},
		{
			Name: "tls",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--tls", "--externaldns"},
		},
		{
			Name: "tls-custom-secret",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--tls", "--tls-secret", "my-tls-secret"},
		},
		{
			Name: "istio",
			Args: []string{"--provider", "kind", "--env-git-public", "--git-public", "--ingress-kind=istio"},
		},
		{
			Name: "kubernetes",
			Args: []string{"--provider", "kubernetes", "--env-git-public", "--git-public"},
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
		args := append(tc.Args, "--git-server", "https://fake.com", "--git-kind", "fake", "--env-git-owner", "jstrachan", "--cluster", tc.Name, "--out", outFileName)
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

		switch tc.Name {
		case "remote":
			AssertHasApp(t, apps, "jenkins-x/chartmuseum", appMessage)
			AssertHasApp(t, apps, "jenkins-x/nexus", appMessage)
			AssertHasApp(t, apps, "repositories", appMessage)
			AssertNoApp(t, apps, "jenkins-x/bucketrepo", appMessage)

		case "bucketrepo":
			AssertHasApp(t, apps, "jenkins-x/bucketrepo", appMessage)
			AssertHasApp(t, apps, "repositories", appMessage)
			AssertNoApp(t, apps, "jenkins-x/chartmuseum", appMessage)
			AssertNoApp(t, apps, "jenkins-x/nexus", appMessage)

		case "tls":
			AssertHasApp(t, apps, "jetstack/cert-manager", appMessage)
			AssertHasApp(t, apps, "bitnami/external-dns", appMessage)
			AssertHasApp(t, apps, "jenkins-x/acme", appMessage)

		case "tls-custom-secret":
			AssertNoApp(t, apps, "jetstack/cert-manager", appMessage)
			AssertNoApp(t, apps, "bitnami/external-dns", appMessage)
			AssertNoApp(t, apps, "jenkins-x/acme", appMessage)

		case "istio":
			AssertHasApp(t, apps, "jx-labs/istio", appMessage)
			AssertNoApp(t, apps, "stable/nginx-ingress", appMessage)

		case "kubernetes":
			AssertHasApp(t, apps, "stable/docker-registry", appMessage)
		}
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

		for i, e := range requirements.Environments {
			if e.Key == "dev" {
				assert.Equal(t, false, e.RemoteCluster, "requirements.Environments[%d].RemoteCluster for key %s", i, e.Key)
			} else {
				expectedRemote := tc.Name == "remote"
				assert.Equal(t, expectedRemote, e.RemoteCluster, "requirements.Environments[%d].RemoteCluster for key %s", i, e.Key)
			}
			t.Logf("requirements.Environments[%d].RemoteCluster = %v for key %s ", i, e.RemoteCluster, e.Key)
		}

		if requirements.Cluster.Provider == "kind" {
			assert.Equal(t, true, requirements.Ingress.IgnoreLoadBalancer, "dev requirements.Ingress.IgnoreLoadBalancer for test %s", tc.Name)
		}
	}
}

// AssertHasApp asserts that the given app name is in the generated apps YAML
func AssertHasApp(t *testing.T, appConfig *config.AppConfig, appName string, message string) {
	found, names := HasApp(t, appConfig, appName, message)
	if !found {
		assert.Fail(t, fmt.Sprintf("does not have the app %s for %s. Current apps are: %s", appName, message, strings.Join(names, ", ")))
	}
}

// AssertNoApp asserts that the given app name is in the generated apps YAML
func AssertNoApp(t *testing.T, appConfig *config.AppConfig, appName string, message string) {
	found, names := HasApp(t, appConfig, appName, message)
	if found {
		assert.Fail(t, fmt.Sprintf("should not have the app %s for %s. Current apps are: %s", appName, message, strings.Join(names, ", ")))
	}
}

// HasApp tests that the app config has the given app
func HasApp(t *testing.T, appConfig *config.AppConfig, appName string, message string) (bool, []string) {
	found := false
	names := []string{}
	for _, app := range appConfig.Apps {
		names = append(names, app.Name)
		if app.Name == appName {
			t.Logf("has app %s for %s", appName, message)
			found = true
		}
	}
	return found, names
}
