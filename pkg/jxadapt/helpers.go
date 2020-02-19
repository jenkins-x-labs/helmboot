package jxadapt

import (
	"os"

	"github.com/jenkins-x-labs/helmboot/pkg/fakes/fakeclientsfactory"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/factory"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/pkg/errors"
)

// JXAdapter an adapter between new clean code and the classic CommonOptions abstractions in jx
// to allow us to move new code away from CommonOptions while reusing existing code
type JXAdapter struct {
	JXFactory jxfactory.Factory
	Gitter    gits.Gitter
	BatchMode bool
}

// NewJXAdapter creates a new adapter
func NewJXAdapter(f jxfactory.Factory, gitter gits.Gitter, batch bool) *JXAdapter {
	if f == nil {
		f = jxfactory.NewFactory()
	}
	return &JXAdapter{
		JXFactory: f,
		Gitter:    gitter,
		BatchMode: batch,
	}
}

// NewCommonOptions creates a CommonOptions that can be used for integrating with jx code which uses it
func (a *JXAdapter) NewCommonOptions() *opts.CommonOptions {
	f := clients.NewUsingFactory(a.JXFactory)
	co := opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)

	cf := fakeclientsfactory.NewFakeFactory(a.JXFactory, f)
	co.SetFactory(cf)

	kubeClient, ns, err := a.JXFactory.CreateKubeClient()
	if err == nil {
		co.SetDevNamespace(ns)
		co.SetKubeClient(kubeClient)
	}
	jxClient, _, err := a.JXFactory.CreateJXClient()
	if err == nil {
		co.SetJxClient(jxClient)
	}
	co.SetKube(a.JXFactory.KubeConfig())

	co.BatchMode = a.BatchMode
	co.SetGit(a.Gitter)
	return co
}

// ScmClient creates a new Scm client for the given git server, owner and kind
func (a *JXAdapter) ScmClient(serverURL string, owner string, kind string) (*scm.Client, string, error) {
	token, defaultKind, err := a.FindGitTokenForServer(serverURL, owner)
	if err != nil {
		return nil, token, err
	}

	if kind == "" {
		kind = defaultKind
	}
	client, err := factory.NewClient(kind, serverURL, token)
	return client, token, err
}

// ScmClientForRepository creates a new Scm client for the given git repository
func (a *JXAdapter) ScmClientForRepository(gitInfo *gits.GitRepository) (*scm.Client, string, error) {
	serverURL := gitInfo.HostURLWithoutUser()

	token, kind, err := a.FindGitTokenForServer(serverURL, gitInfo.Organisation)
	if err != nil {
		return nil, token, err
	}

	client, err := factory.NewClient(kind, serverURL, token)
	return client, token, err
}

// FindGitTokenForServer finds the git token and kind for the given server URL
func (a *JXAdapter) FindGitTokenForServer(serverURL string, owner string) (string, string, error) {
	co := a.NewCommonOptions()
	authSvc, err := co.GitLocalAuthConfigService()
	token := ""
	kind := ""
	if err != nil {
		return token, kind, errors.Wrapf(err, "failed to load local git auth")
	}
	cfg, err := authSvc.LoadConfig()
	if err != nil {
		return token, kind, errors.Wrapf(err, "failed to load local git auth config")
	}
	server := cfg.GetOrCreateServer(serverURL)
	kind = server.Kind
	if kind == "" {
		kind = gits.SaasGitKind(serverURL)
	}

	userAuth, err := cfg.PickServerUserAuth(server, "Git user name:", a.BatchMode, owner, co.GetIOFileHandles())
	if err != nil {
		return token, kind, err
	}

	if userAuth == nil || userAuth.IsInvalid() {
		return token, kind, errors.Wrapf(err, "no valid token setup for git server %s", serverURL)
	}
	token = userAuth.ApiToken
	if token == "" {
		token = userAuth.BearerToken
	}
	return token, kind, nil
}
