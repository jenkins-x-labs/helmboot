package githelpers

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/google/uuid"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// AddAndCommitFiles add and commits files
func AddAndCommitFiles(gitter gits.Gitter, dir string, message string) (bool, error) {
	err := gitter.Add(dir, "*")
	if err != nil {
		return false, errors.Wrapf(err, "failed to add files to git")
	}
	changes, err := gitter.HasChanges(dir)
	if err != nil {
		if err != nil {
			return changes, errors.Wrapf(err, "failed to check if there are changes")
		}
	}
	if !changes {
		return changes, nil
	}
	err = gitter.CommitDir(dir, message)
	if err != nil {
		return changes, errors.Wrapf(err, "failed to git commit initial code changes")
	}
	return changes, nil
}

// CreateBranch creates a dynamic branch name and branch
func CreateBranch(gitter gits.Gitter, dir string) (string, error) {
	branchName := fmt.Sprintf("pr-%s", uuid.New().String())
	gitRef := branchName
	err := gitter.CreateBranch(dir, branchName)
	if err != nil {
		return branchName, errors.Wrapf(err, "create branch %s from %s", branchName, gitRef)
	}

	err = gitter.Checkout(dir, branchName)
	if err != nil {
		return branchName, errors.Wrapf(err, "checkout branch %s", branchName)
	}
	return branchName, nil
}

// GitCloneToTempDir clones the git repository to either the given directory or create a temporary
func GitCloneToTempDir(gitter gits.Gitter, gitURL string, dir string) (string, error) {
	var err error
	if dir != "" {
		err = os.MkdirAll(dir, util.DefaultWritePermissions)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create directory %s", dir)
		}
	} else {
		dir, err = ioutil.TempDir("", "helmboot-")
		if err != nil {
			return "", errors.Wrap(err, "failed to create temporary directory")
		}
	}

	log.Logger().Debugf("cloning %s to directory %s", util.ColorInfo(gitURL), util.ColorInfo(dir))

	err = gitter.Clone(gitURL, dir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone repository %s to directory: %s", gitURL, dir)
	}
	return dir, nil
}
