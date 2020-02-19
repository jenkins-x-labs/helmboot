package fakegit

import "github.com/jenkins-x/jx/pkg/gits"

// GitFakeClone struct for the fake git
type GitFakeClone struct {
	gits.GitFake
}

// NewGitFakeClone a fake Gitter but implements cloning
func NewGitFakeClone() gits.Gitter {
	return &GitFakeClone{}
}

func (f *GitFakeClone) Clone(url string, directory string) error {
	return gits.NewGitCLI().Clone(url, directory)
}
