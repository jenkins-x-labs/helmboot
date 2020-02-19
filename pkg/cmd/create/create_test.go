package create_test

import (
	"context"
	"testing"

	"github.com/jenkins-x-labs/helmboot/pkg/cmd/create"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakegit"
	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakejxfactory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	//t.Parallel()

	_, co := create.NewCmdCreate()
	co.BatchMode = true
	co.Gitter = fakegit.NewGitFakeClone()
	co.Args = []string{"--provider", "kubernetes", "--cluster", "mycluster", "--git-server", "https://fake.com", "--git-kind", "fake", "--env-git-owner", "jstrachan"}
	co.JXFactory = fakejxfactory.NewFakeFactory()

	err := co.Run()
	require.NoError(t, err, "failed to create repository")

	// now lets assert we created a new repository
	ctx := context.Background()
	fullName := "jstrachan/environment-mycluster-dev"
	repo, _, err := co.EnvFactory.ScmClient.Repositories.Find(ctx, fullName)
	require.NoError(t, err, "failed to find repository %s", fullName)
	assert.NotNil(t, repo, "nil repository %s", fullName)
	assert.Equal(t, fullName, repo.FullName, "repo.FullName")
	assert.Equal(t, "environment-mycluster-dev", repo.Name, "repo.FullName")

}
