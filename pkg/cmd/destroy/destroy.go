package destroy

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/githelpers"
	"github.com/jenkins-x-labs/helmboot/pkg/reqhelpers"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/create/helmfile"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Options contains the command line arguments for this command
type Options struct {
	CreateHelmfileOptions helmfile.CreateHelmfileOptions
	JXFactory             jxfactory.Factory
	Gitter                gits.Gitter
	Dir                   string
	GitURL                string
	BatchMode             bool
}

var (
	destroyLong = templates.LongDesc(`
		This command destroys all of the charts installed via the 'jx-apps.yml' file

`)

	destroyExample = templates.Examples(`
		# destroy the helm charts installed via 'jx-apps.yml'
		%s destroy 
`)

	dummySecretYaml = `foo: bar`
)

// NewCmdDestroy creates the command
func NewCmdDestroy() *cobra.Command {
	options := Options{}
	command := &cobra.Command{
		Use:     "destroy",
		Short:   "destroys all of the charts installed via the 'jx-apps.yml' file",
		Long:    destroyLong,
		Example: fmt.Sprintf(destroyExample, common.BinaryName, common.BinaryName),
		Run: func(command *cobra.Command, args []string) {
			common.SetLoggingLevel(command, args)
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.GitURL, "git-url", "u", "", "override the Git clone URL for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-ref as well")
	command.Flags().BoolVarP(&options.BatchMode, "batch-mode", "b", false, "Runs in batch mode without prompting for user input")

	return command
}

// Run implements the command
func (o *Options) Run() error {
	if o.CreateHelmfileOptions.CommonOptions == nil {
		f := clients.NewFactory()
		o.CreateHelmfileOptions.CommonOptions = opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
		o.CreateHelmfileOptions.CommonOptions.BatchMode = o.BatchMode
	}
	if o.JXFactory == nil {
		o.JXFactory = jxfactory.NewFactory()
	}
	gitURL, err := reqhelpers.FindGitURL(o.JXFactory)
	if err != nil {
		return errors.Wrap(err, "failed to find Git URL")
	}

	dir, err := githelpers.GitCloneToTempDir(o.Git(), gitURL, o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to clone Git URL %s", gitURL)
	}

	o.CreateHelmfileOptions.Dir = dir
	o.CreateHelmfileOptions.IgnoreNamespaceCheck = true
	err = o.CreateHelmfileOptions.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to generate the helmfiles to %s", dir)
	}

	if !o.BatchMode {
		c, err := util.Confirm("You are about to destroy your boot installation. Are you sure?", false, "Destroying your installation will preserve your kubernetes cluster and the underlying cloud resources so you can re-run boot again", o.CreateHelmfileOptions.CommonOptions.GetIOFileHandles())
		if err != nil {
			return err
		}
		if !c {
			return nil
		}
	}

	log.Logger().Infof("destroying the boot installation using temporary dir: %s", dir)

	secretsYaml := filepath.Join(dir, "secrets.yaml")
	err = ioutil.WriteFile(secretsYaml, []byte(dummySecretYaml), util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save dummy secrets at %s", secretsYaml)
	}

	env := map[string]string{
		"JX_SECRETS_YAML": secretsYaml,
	}
	log.Logger().Infof("removing the apps charts...")
	err = o.runCommand(filepath.Join(dir, "apps"), env, "helmfile", "destroy")
	if err != nil {
		return err
	}

	log.Logger().Infof("removing the system charts...")
	err = o.runCommand(filepath.Join(dir, "system"), env, "helmfile", "destroy")
	if err != nil {
		return err
	}

	log.Logger().Infof("chart removal complete. You can run 'jxl boot run' to reinstall")
	return nil
}

// Git lazily create a gitter if its not specified
func (o *Options) Git() gits.Gitter {
	if o.Gitter == nil {
		o.Gitter = gits.NewGitCLI()
	}
	return o.Gitter
}

func (o *Options) runCommand(dir string, env map[string]string, cmd string, args ...string) error {
	exists, err := util.DirExists(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to check dir exists: %s", dir)
	}
	if !exists {
		return fmt.Errorf("directory does not exist %s", dir)
	}
	c := util.Command{
		Name: cmd,
		Args: args,
		Dir:  dir,
		Env:  env,
	}
	_, err = c.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to run command: %s %s in dir %s", cmd, strings.Join(args, " "), dir)
	}
	return nil
}
