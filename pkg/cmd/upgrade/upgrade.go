package upgrade

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/uuid"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/create"
	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/envfactory"
	"github.com/jenkins-x-labs/helmboot/pkg/upgrader"
	"github.com/jenkins-x/go-scm/scm"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	upgradeLong = templates.LongDesc(`
		Upgrades your Jenkins X installation to helm 3 / helmfile.

		This command will by default create a temporary directory and populate it with the source code to replicate the 
		current Jenkins X installation using helmboot, helmfile and helm 3.

		If your cluster was created via GitOps then a Pull Request is created to upgrade the git repository to use helmfile / helm 3.

		Otherwise a new git repository is created
`)

	upgradeExample = templates.Examples(`
		# upgrades your current cluster of Jenkins X to helm 3 / helmfile
		%s upgrade
	`)
)

// UpgradeOptions the options for viewing running PRs
type UpgradeOptions struct {
	envfactory.EnvFactory

	OverrideRequirements config.RequirementsConfig
	Namespace            string
	GitCloneURL          string
	InitialGitURL        string
	Dir                  string
	NoCommit             bool

	// if we are modifing an existing git repository
	gitRepositoryExisted bool
	branchName           string
}

// NewCmdUpgrade creates a command object for the "create" command
func NewCmdUpgrade() (*cobra.Command, *UpgradeOptions) {
	o := &UpgradeOptions{}

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrades your Development environmentsgit repository to use helmfile and helm 3",
		Long:    upgradeLong,
		Example: fmt.Sprintf(upgradeExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	o.AddUpgradeOptions(cmd)
	return cmd, o
}

func (o *UpgradeOptions) AddUpgradeOptions(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", "", "The directory used to create/clone the development git repository. If no directory is specified a temporary directory will be used")
	cmd.Flags().StringVarP(&o.GitCloneURL, "git-url", "g", "", "The git repository to clone to upgrade")
	cmd.Flags().StringVarP(&o.InitialGitURL, "initial-git-url", "", common.DefaultBootHelmfileRepository, "The git URL to clone to fetch the initial set of files for a helm 3 / helmfile based git configuration if this command is not run inside a git clone or against a GitOps based cluster")

	create.AddGitRequirementsOptions(cmd, &o.OverrideRequirements)

	o.EnvFactory.AddFlags(cmd)
}

// Run implements the command
func (o *UpgradeOptions) Run() error {
	u, jxClient, ns, err := o.createUpgrader()
	if err != nil {
		return err
	}
	req, err := u.ExportRequirements()
	if err != nil {
		return errors.Wrapf(err, "failed to generate the JX Requirements from the cluster")
	}

	dir, err := o.gitCloneIfRequired(o.Gitter, u.DevSource)
	if err != nil {
		return err
	}

	if o.gitRepositoryExisted {
		err = o.createBranch(o.Gitter, dir)
		if err != nil {
			return errors.Wrapf(err, "failed to create git branch in %s", dir)
		}
	}

	reqFile := filepath.Join(dir, config.RequirementsConfigFileName)
	err = req.SaveConfig(reqFile)
	if err != nil {
		return errors.Wrapf(err, "failed to save migrated requirements file %s", reqFile)
	}

	err = o.replacePipeline(dir)
	if err != nil {
		return err
	}

	err = o.removeGeneratedRequirementsValuesFile(dir)
	if err != nil {
		return err
	}

	log.Logger().Infof("generated the latest cluster requirements configuration to %s", util.ColorInfo(reqFile))

	err = o.addAndRemoveFiles(dir, jxClient, ns)
	if err != nil {
		return err
	}

	log.Logger().Infof("generated the boot configuration from the current cluster into the directory: %s", util.ColorInfo(dir))

	// now lets add the generated files to git
	err = o.Gitter.Add(dir, "*")
	if err != nil {
		return errors.Wrapf(err, "failed to add files to git")
	}

	if o.gitRepositoryExisted {
		o.OutDir = dir
		if !o.NoCommit {
			err = o.Gitter.CommitIfChanges(dir, "fix: helmboot upgrader\n\nmigrating resources across to the latest Jenkins X GitOps source code")
			if err != nil {
				return errors.Wrapf(err, "failed to git commit changes")
			}

			if o.GitCloneURL != "" {
				return o.createPullRequest(dir, u)
			}

			return o.EnvFactory.PrintBootJobInstructions(req, o.GitCloneURL)
		}
		return nil
	}
	return o.EnvFactory.CreateDevEnvGitRepository(dir)
}

func (o *UpgradeOptions) removeGeneratedRequirementsValuesFile(dir string) error {
	// lets remove the extra yaml file used during the boot process (we should disable this via a flag via changing the jx code)
	requirementsValuesFile := filepath.Join(dir, config.RequirementsValuesFileName)
	exists, err := util.FileExists(requirementsValuesFile)
	if err != nil {
		return errors.Wrapf(err, "failed to check requirements values file exists %s", requirementsValuesFile)
	}
	if exists {
		err = os.Remove(requirementsValuesFile)
		if err != nil {
			return errors.Wrapf(err, "failed to remove file %s", requirementsValuesFile)
		}
	}
	return nil
}

func (o *UpgradeOptions) createUpgrader() (*upgrader.HelmfileUpgrader, versioned.Interface, string, error) {
	if o.Gitter == nil {
		o.Gitter = gits.NewGitCLI()
	}

	if o.JXFactory == nil {
		o.JXFactory = jxfactory.NewFactory()
	}

	jxClient, ns, err := o.JXFactory.CreateJXClient()
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "failed to connect to the Kubernetes cluster")
	}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, "", errors.Wrapf(err, "failed to list Jenkins X Environments in namespace %s", ns)
	}

	u := &upgrader.HelmfileUpgrader{
		Environments:         envs.Items,
		OverrideRequirements: &o.OverrideRequirements,
	}
	return u, jxClient, ns, nil
}

func (o *UpgradeOptions) addAndRemoveFiles(dir string, jxClient versioned.Interface, ns string) error {
	err := o.removeOldDirs(dir)
	if err != nil {
		return err
	}

	err = o.addMissingFiles(dir)
	if err != nil {
		return err
	}

	srOutDir := filepath.Join(dir, "repositories", "templates")
	err = os.MkdirAll(srOutDir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create the SourceRepository output directory: %s", srOutDir)
	}

	err = o.writeAdditionalHelmTemplateFiles(jxClient, ns, srOutDir)
	if err != nil {
		return errors.Wrapf(err, "failed to write additional helm template files")
	}

	err = o.writeNonHelmManagedResources(jxClient, ns, dir)
	if err != nil {
		return errors.Wrapf(err, "failed to write migration files")
	}
	return nil
}

// addMissingFiles if the current dir is an old helm 2 style repository
// lets copy across any new directories/files from the template git repository
func (o *UpgradeOptions) addMissingFiles(dir string) error {
	templateDir := ""
	lazyCloneTemplates := func() error {
		if templateDir == "" {
			dir, err := ioutil.TempDir("", "helmboot-init")
			if err != nil {
				return errors.Wrap(err, "failed to create temp dir")
			}
			err = o.Gitter.Clone(o.InitialGitURL, dir)
			if err != nil {
				return errors.Wrapf(err, "failed to git clone %s", o.InitialGitURL)
			}
			templateDir = dir
		}
		return nil
	}

	files := []string{"environments.yaml", "helmfile.yaml", "jx-apps.yml"}
	dirs := []string{"apps", "repositories", "system"}
	for _, name := range dirs {
		d := filepath.Join(dir, name)
		exists, err := util.DirExists(d)
		if err != nil {
			return errors.Wrapf(err, "failed to check dir exists %s", d)
		}
		if !exists {
			err = lazyCloneTemplates()
			if err != nil {
				return err
			}
			err = util.CopyDirOverwrite(filepath.Join(templateDir, name), d)
			if err != nil {
				return errors.Wrapf(err, "failed to copy missing dir %s", d)
			}
		}
	}
	for _, name := range files {
		f := filepath.Join(dir, name)
		exists, err := util.FileExists(f)
		if err != nil {
			return errors.Wrapf(err, "failed to check file exists %s", f)
		}
		if !exists {
			err = lazyCloneTemplates()
			if err != nil {
				return err
			}
			err = util.CopyFile(filepath.Join(templateDir, name), f)
			if err != nil {
				return errors.Wrapf(err, "failed to copy missing file %s", f)
			}
		}
	}
	return nil
}

// removeOldDirs lets remove any old files/directories from the helm 2.x style git repository
func (o *UpgradeOptions) removeOldDirs(dir string) error {
	oldDirs := []string{"env", "systems", "kubeProviders", "prowConfig"}
	for _, od := range oldDirs {
		oldDir := filepath.Join(dir, od)
		exists, err := util.DirExists(oldDir)
		if err != nil {
			return errors.Wrapf(err, "failed to check dir exists %s", oldDir)
		}
		if exists {
			err = os.RemoveAll(oldDir)
			if err != nil {
				return errors.Wrapf(err, "failed to remove dir %s", oldDir)
			}
			log.Logger().Infof("removed old folder %s", oldDir)
		}
	}
	return nil
}

// writeAdditionalHelmTemplateFiles lets store to git any extra resources managed outside of the regular boot charts
func (o *UpgradeOptions) writeAdditionalHelmTemplateFiles(jxClient versioned.Interface, ns string, outDir string) error {
	// lets write the SourceRepository resources to the repositories folder...
	srList, err := jxClient.JenkinsV1().SourceRepositories(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to query the SourceRepository resources in namespace %s", ns)
	}
	_, err = upgrader.WriteSourceRepositoriesToGitFolder(outDir, srList)
	if err != nil {
		return errors.Wrapf(err, "failed to write SourceRepository resources to %s", outDir)
	}
	return nil
}

// writeAdditionalHelmTemplateFiles lets store to git any extra resources managed outside of the regular boot charts
func (o *UpgradeOptions) writeNonHelmManagedResources(jxClient versioned.Interface, ns string, dir string) error {
	paList, err := jxClient.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to query the PipelineActivity resources in namespace %s", ns)
	}
	if len(paList.Items) == 0 {
		return nil
	}

	// lets clear out unnecessary metadata
	paList.ListMeta = metav1.ListMeta{}
	for i := range paList.Items {
		pa := &paList.Items[i]
		pa.ObjectMeta = upgrader.EmptyObjectMeta(&pa.ObjectMeta)
	}

	data, err := yaml.Marshal(paList)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal PipelineActivity resources to YAML")
	}

	fileName := filepath.Join(dir, common.PipelineActivitiesYAMLFile)
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %s", fileName)
	}
	log.Logger().Infof("wrote migration resources file: %s", util.ColorInfo(common.PipelineActivitiesYAMLFile))
	return nil
}

// gitCloneIfRequired if the specified directory is already a git clone then lets just use it
// otherwise lets make a temporary directory and clone the git repository specified
// or if there is none make a new one
func (o *UpgradeOptions) gitCloneIfRequired(gitter gits.Gitter, devSource v1.EnvironmentRepository) (string, error) {
	o.gitRepositoryExisted = true
	gitURL := o.GitCloneURL
	if gitURL == "" {
		gitURL = devSource.URL
		o.GitCloneURL = gitURL
		if gitURL == "" {
			gitURL = o.InitialGitURL
			o.gitRepositoryExisted = false
		}
	}
	var err error
	dir := o.Dir
	if dir == "" {
		dir, err = ioutil.TempDir("", "helmboot-")
		if err != nil {
			return "", errors.Wrap(err, "failed to create temporary directory")
		}
	} else {
		// if you specify and it has a git clone inside lets just use it rather than cloning
		// as you may be inside a fork or something
		d, _, err := gitter.FindGitConfigDir(dir)
		if err != nil {
			return "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
		}
		if d != "" {
			o.gitRepositoryExisted = true
			return dir, nil
		}
	}

	log.Logger().Infof("cloning %s to directory %s", util.ColorInfo(gitURL), util.ColorInfo(dir))

	err = gitter.Clone(gitURL, dir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone repository %s to directory: %s", gitURL, dir)
	}
	return dir, nil
}

func (o *UpgradeOptions) createBranch(gitter gits.Gitter, dir string) error {
	o.branchName = fmt.Sprintf("pr-%s", uuid.New().String())
	gitRef := o.branchName
	err := gitter.CreateBranch(dir, o.branchName)
	if err != nil {
		return errors.Wrapf(err, "create branch %s from %s", o.branchName, gitRef)
	}

	err = gitter.Checkout(dir, o.branchName)
	if err != nil {
		return errors.Wrapf(err, "checkout branch %s", o.branchName)
	}
	return nil
}

// replacePipeline if the `jenkins-x.yml` file is missing or does use the helm 3 / helmfile style configuration
// lets replace with the new pipeline file
func (o *UpgradeOptions) replacePipeline(dir string) error {
	projectConfig, fileName, err := config.LoadProjectConfig(dir)
	if err != nil {
		return errors.Wrap(err, "failed to load Jenkins X Pipeline")
	}
	if projectConfig.BuildPack == common.HelmfileBuildPackName {
		return nil
	}
	projectConfig = &config.ProjectConfig{}
	projectConfig.BuildPack = common.HelmfileBuildPackName

	err = projectConfig.SaveConfig(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to save Jenkins X Pipeline")
	}
	return nil
}

func (o *UpgradeOptions) createPullRequest(dir string, u *upgrader.HelmfileUpgrader) error {
	remote := "origin"
	err := o.Gitter.Push(dir, remote, false)
	if err != nil {
		return errors.Wrapf(err, "failed to push to remote %s from dir %s", remote, dir)
	}

	gitURL := o.GitCloneURL
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL")
	}

	serverURL := gitInfo.HostURLWithoutUser()
	owner := gitInfo.Organisation
	kind := u.GitKind()

	scmClient, _, err := o.EnvFactory.JXAdapter().ScmClient(serverURL, owner, kind)
	if err != nil {
		return errors.Wrapf(err, "failed to create SCM client for %s", gitURL)
	}
	o.ScmClient = scmClient

	headPrefix := ""
	// if username is a fork then
	//	headPrefix = username + ":"

	head := headPrefix + o.branchName

	ctx := context.Background()
	pri := &scm.PullRequestInput{
		Title: "fix: upgrade to helmfile + helm 3",
		Head:  head,
		Base:  "master",
		Body:  "",
	}
	repoFullName := scm.Join(gitInfo.Organisation, gitInfo.Name)
	pr, _, err := scmClient.PullRequests.Create(ctx, repoFullName, pri)
	if err != nil {
		return errors.Wrapf(err, "failed to create PullRequest on %s", gitURL)
	}

	// the URL should not really end in .diff - fix in go-scm
	link := strings.TrimSuffix(pr.Link, ".diff")
	log.Logger().Infof("created Pull Request %s", util.ColorInfo(link))
	return nil
}
