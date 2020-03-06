package create

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/envfactory"
	"github.com/jenkins-x-labs/helmboot/pkg/githelpers"
	"github.com/jenkins-x-labs/helmboot/pkg/reqhelpers"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

var (
	createLong = templates.LongDesc(`
		Creates a new git repository for a new Jenkins X installation
`)

	createExample = templates.Examples(`
		# create a new git repository which we can then boot up
		%s create
	`)
)

// CreateOptions the options for creating a repository
type CreateOptions struct {
	envfactory.EnvFactory
	DisableVerifyPackages bool
	Requirements          config.RequirementsConfig
	Flags                 reqhelpers.RequirementBools
	InitialGitURL         string
	Dir                   string
	Cmd                   *cobra.Command
	Args                  []string
}

// NewCmdCreate creates a command object for the "create" command
func NewCmdCreate() (*cobra.Command, *CreateOptions) {
	o := &CreateOptions{}

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Creates a new git repository for a new Jenkins X installation",
		Long:    createLong,
		Example: fmt.Sprintf(createExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd = cmd
			o.Args = args
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	o.Cmd = cmd

	cmd.Flags().StringVarP(&o.InitialGitURL, "initial-git-url", "", "", "The git URL to clone to fetch the initial set of files for a helm 3 / helmfile based git configuration if this command is not run inside a git clone or against a GitOps based cluster")
	cmd.Flags().StringVarP(&o.Dir, "dir", "", "", "The directory used to create the development environment git repository inside. If not specified a temporary directory will be used")

	reqhelpers.AddRequirementsFlagsOptions(cmd, &o.Flags)
	reqhelpers.AddRequirementsOptions(cmd, &o.Requirements)

	o.EnvFactory.AddFlags(cmd)
	return cmd, o
}

// Run implements the command
func (o *CreateOptions) Run() error {
	if o.Gitter == nil {
		o.Gitter = gits.NewGitCLI()
	}

	dir, err := o.gitCloneIfRequired(o.Gitter)
	if err != nil {
		return err
	}

	err = reqhelpers.OverrideRequirements(o.Cmd, o.Args, dir, &o.Requirements, &o.Flags)
	if err != nil {
		return errors.Wrapf(err, "failed to override requirements in dir %s", dir)
	}

	err = o.EnvFactory.VerifyPreInstall(o.DisableVerifyPackages, dir)
	if err != nil {
		return errors.Wrapf(err, "failed to verify requirements in dir %s", dir)
	}

	log.Logger().Infof("created git source at %s", util.ColorInfo(dir))

	_, err = githelpers.AddAndCommitFiles(o.Gitter, dir, "fix: initial code")
	if err != nil {
		return err
	}
	return o.EnvFactory.CreateDevEnvGitRepository(dir)
}

// gitCloneIfRequired if the specified directory is already a git clone then lets just use it
// otherwise lets make a temporary directory and clone the git repository specified
// or if there is none make a new one
func (o *CreateOptions) gitCloneIfRequired(gitter gits.Gitter) (string, error) {
	gitURL := o.InitialGitURL
	if gitURL == "" {
		gitURL = common.DefaultBootHelmfileRepository
	}
	var err error
	dir := o.Dir
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
