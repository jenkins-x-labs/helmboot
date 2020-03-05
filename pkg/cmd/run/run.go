package run

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/jxadapt"
	"github.com/jenkins-x-labs/helmboot/pkg/reqhelpers"
	"github.com/jenkins-x/jx/pkg/cmd/boot"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// RunOptions contains the command line arguments for this command
type RunOptions struct {
	boot.BootOptions
	JXFactory jxfactory.Factory
	Gitter    gits.Gitter
	BatchMode bool
	JobMode   bool
}

var (
	stepCustomPipelineLong = templates.LongDesc(`
		This command boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps

`)

	stepCustomPipelineExample = templates.Examples(`
		# runs the boot Job to install for the first time
		%s run --git-url https://github.com/myorg/environment-mycluster-dev.git

		# runs the boot Job to upgrade a cluster from the latest in git
		%s run 
`)
)

// NewCmdRun creates the new command
func NewCmdRun() *cobra.Command {
	options := RunOptions{}
	command := &cobra.Command{
		Use:     "run",
		Short:   "boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps by triggering a Kubernetes Job inside the cluster",
		Long:    stepCustomPipelineLong,
		Example: fmt.Sprintf(stepCustomPipelineExample, common.BinaryName, common.BinaryName),
		Run: func(command *cobra.Command, args []string) {
			common.SetLoggingLevel(command, args)
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the Jenkins X Pipeline, requirements and charts")
	command.Flags().StringVarP(&options.GitURL, "git-url", "u", "", "override the Git clone URL for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-ref as well")
	command.Flags().StringVarP(&options.GitRef, "git-ref", "", "master", "override the Git ref for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-url as well")
	command.Flags().StringVarP(&options.VersionStreamURL, "versions-repo", "", common.DefaultVersionsURL, "the bootstrap URL for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	command.Flags().StringVarP(&options.VersionStreamRef, "versions-ref", "", common.DefaultVersionsRef, "the bootstrap ref for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	command.Flags().StringVarP(&options.HelmLogLevel, "helm-log", "v", "", "sets the helm logging level from 0 to 9. Passed into the helm CLI via the '-v' argument. Useful to diagnose helm related issues")
	command.Flags().StringVarP(&options.RequirementsFile, "requirements", "r", "", "requirements file which will overwrite the default requirements file")

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	command.PersistentFlags().BoolVarP(&options.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")

	command.Flags().BoolVarP(&options.JobMode, "job", "", false, "if running inside the cluster lets still default to creating the boot Job rather than running boot locally")

	return command
}

// Run implements the command
func (o *RunOptions) Run() error {
	if o.JobMode || !IsInCluster() {
		return o.RunBootJob()
	}
	bo := &o.BootOptions
	if bo.CommonOptions == nil {
		f := clients.NewFactory()
		bo.CommonOptions = opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
		bo.BatchMode = o.BatchMode
	}
	return bo.Run()
}

// RunBootJob runs the boot installer Job
func (o *RunOptions) RunBootJob() error {
	requirements, gitURL, err := o.findRequirementsAndGitURL()
	if err != nil {
		return err
	}
	if gitURL == "" {
		return util.MissingOption("git-url")
	}

	clusterName := requirements.Cluster.ClusterName
	log.Logger().Infof("running helmboot Job for cluster %s with git URL %s", util.ColorInfo(clusterName), util.ColorInfo(gitURL))

	// TODO while the chart is released lets do a local clone....
	tempDir, err := ioutil.TempDir("", "jx-boot-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}

	installerGitURL := "https://github.com/jenkins-x-labs/jenkins-x-installer.git"
	log.Logger().Debugf("cloning %s to %s", installerGitURL, tempDir)
	err = o.Git().Clone(installerGitURL, tempDir)
	if err != nil {
		return errors.Wrapf(err, "failed to git clone %s to dir %s", installerGitURL, tempDir)
	}

	flag, err := o.hasHelmRelease("jx-boot")
	if err != nil {
		return err
	}
	if flag {
		log.Logger().Info("uninstalling old jx-boot chart ...")
		c := util.Command{
			Dir:  tempDir,
			Name: "helm",
			Args: []string{"uninstall", "jx-boot"},
		}
		_, err = c.RunWithoutRetry()
		if err != nil {
			return errors.Wrapf(err, "failed to remove old jx-boot chart")
		}
	}

	c := reqhelpers.GetBootJobCommand(requirements, gitURL)
	c.Dir = tempDir

	commandLine := fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))

	log.Logger().Infof("running the command:\n\n%s\n\n", util.ColorInfo(commandLine))

	_, err = c.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to run command %s", commandLine)
	}

	return o.tailJobLogs()
}

func (o *RunOptions) tailJobLogs() error {
	a := jxadapt.NewJXAdapter(o.JXFactory, o.Git(), o.BatchMode)
	client, ns, err := o.JXFactory.CreateKubeClient()
	if err != nil {
		return err
	}
	co := a.NewCommonOptions()

	selector := map[string]string{
		"job-name": "jx-boot",
	}
	containerName := "boot"
	podInterface := client.CoreV1().Pods(ns)
	for {
		pod := ""
		if err != nil {
			return err
		}
		pod, err = co.WaitForReadyPodForSelectorLabels(client, ns, selector, false)
		if err != nil {
			return err
		}
		if pod == "" {
			return fmt.Errorf("No pod found for namespace %s with selector %v", ns, selector)
		}
		err = co.TailLogs(ns, pod, containerName)
		if err != nil {
			return nil
		}
		podResource, err := podInterface.Get(pod, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to get pod %s in namespace %s", pod, ns)
		}
		if kube.IsPodCompleted(podResource) {
			log.Logger().Infof("the Job pod %s has completed successfully", pod)
			return nil
		}
		log.Logger().Warnf("Job pod %s is not completed but has status: %s", pod, kube.PodStatus(podResource))
	}
}

func (o *RunOptions) hasHelmRelease(releaseName string) (bool, error) {
	c := util.Command{
		Name: "helm",
		Args: []string{"list", "--short"},
	}
	text, err := c.RunWithoutRetry()
	if err != nil {
		return false, errors.Wrap(err, "failed to run: helm list")
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == releaseName {
			return true, nil
		}
	}
	return false, nil
}

// Git lazily create a gitter if its not specified
func (o *RunOptions) Git() gits.Gitter {
	if o.Gitter == nil {
		o.Gitter = gits.NewGitCLI()
	}
	return o.Gitter
}

// findRequirementsAndGitURL tries to find the current boot configuration from the cluster
func (o *RunOptions) findRequirementsAndGitURL() (*config.RequirementsConfig, string, error) {
	if o.JXFactory == nil {
		o.JXFactory = jxfactory.NewFactory()
	}

	var requirements *config.RequirementsConfig
	gitURL := ""

	jxClient, ns, err := o.JXFactory.CreateJXClient()
	if err != nil {
		return requirements, gitURL, err
	}
	devEnv, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		return requirements, gitURL, err
	}
	if devEnv != nil {
		gitURL = devEnv.Spec.Source.URL
		requirements, err = config.GetRequirementsConfigFromTeamSettings(&devEnv.Spec.TeamSettings)
		if err != nil {
			log.Logger().Debugf("failed to load requirements from team settings %s", err.Error())
		}
	}
	if o.GitURL != "" {
		gitURL = o.GitURL

		if requirements == nil {
			requirements, err = reqhelpers.GetRequirementsFromGit(gitURL)
			if err != nil {
				return requirements, gitURL, errors.Wrapf(err, "failed to get requirements from git URL %s", gitURL)
			}
		}
	}

	if requirements == nil {
		requirements, _, err = config.LoadRequirementsConfig(o.Dir)
		if err != nil {
			return requirements, gitURL, err
		}
	}

	if gitURL == "" {
		// lets try find the git URL from
		gitURL, err = o.findGitURLFromDir()
		if err != nil {
			return requirements, gitURL, errors.Wrapf(err, "your cluster has not been booted before and you are not inside a git clone of your dev environment repository so you need to pass in the URL of the git repository as --git-url")
		}
	}
	return requirements, gitURL, nil
}

func (o *RunOptions) findGitURLFromDir() (string, error) {
	dir := o.Dir
	_, gitConfDir, err := o.Git().FindGitConfigDir(dir)
	if err != nil {
		return "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
	}

	if gitConfDir == "" {
		return "", fmt.Errorf("no .git directory could be found from dir %s", dir)
	}
	return o.Git().DiscoverUpstreamGitURL(gitConfDir)
}

// IsInCluster tells if we are running incluster
func IsInCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}
