package upgrade_test

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/upgrade"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakegit"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakejxfactory"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUpgrade(t *testing.T) {
	//t.Parallel()

	ns := "jx"
	sourceDir := filepath.Join("test_data")

	testDirs, err := ioutil.ReadDir(sourceDir)
	require.NoError(t, err, "failed to read dir %s", sourceDir)
	for _, d := range testDirs {
		name := d.Name()
		if !d.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		t.Logf("running test %s\n", name)

		testDir := filepath.Join(sourceDir, name)
		envDir := filepath.Join(testDir, "env")

		files, err := ioutil.ReadDir(envDir)
		require.NoError(t, err, "failed to read dir %s", envDir)

		kubeObjects := []runtime.Object{}
		jxObjects := []runtime.Object{}
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".yaml" {
				e := &v1.Environment{}
				fileName := filepath.Join(envDir, f.Name())
				t.Logf("loading environment %s", fileName)
				data, err := ioutil.ReadFile(fileName)
				require.NoError(t, err, "failed to load environment %s", fileName)

				err = yaml.Unmarshal(data, e)
				require.NoError(t, err, "failed to unmarshal environment %s", fileName)
				e.Namespace = ns
				jxObjects = append(jxObjects, e)
			}
		}

		_, uo := upgrade.NewCmdUpgrade()
		uo.BatchMode = true
		uo.Gitter = fakegit.NewGitFakeClone()
		uo.JXFactory = fakejxfactory.NewFakeFactoryWithObjects(kubeObjects, jxObjects, ns)

		createRepo := name == "jx-install"
		fullName := "jstrachan/environment-mycluster-dev"

		if createRepo {
			fullName = "myorg/dummy"
			uo.RepoName = "dummy"
			uo.OverrideRequirements.Cluster.GitKind = "fake"
			uo.OverrideRequirements.Cluster.GitServer = "https://fake.com"
		}
		err = uo.Run()
		require.NoError(t, err, "failed to upgrade repository")

		scmClient := uo.EnvFactory.ScmClient
		require.NotNil(t, scmClient, "no ScmClient created")

		ctx := context.Background()
		if createRepo {
			// now lets assert we created a new repository
			repo, _, err := scmClient.Repositories.Find(ctx, fullName)
			require.NoError(t, err, "failed to find repository %s", fullName)
			assert.NotNil(t, repo, "nil repository %s", fullName)
			assert.Equal(t, fullName, repo.FullName, "repo.FullName")
			assert.Equal(t, uo.RepoName, repo.Name, "repo.FullName")
		} else {
			// lets assert we created a Pull Request
			pr, _, err := scmClient.PullRequests.Find(ctx, fullName, 1)
			require.NoError(t, err, "failed to find repository %s", fullName)
			assert.NotNil(t, pr, "nil pr %s", fullName)

			t.Logf("created PullRequest %s", pr.Link)
		}
	}
}
