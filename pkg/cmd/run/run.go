package run

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/jenkins-x-labs/helmboot/pkg/clienthelpers"
	"github.com/jenkins-x-labs/helmboot/pkg/cmd/secrets"
	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x-labs/helmboot/pkg/helmer"
	"github.com/jenkins-x-labs/helmboot/pkg/jxadapt"
	"github.com/jenkins-x-labs/helmboot/pkg/reqhelpers"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr"
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/factory"
	"github.com/jenkins-x/jx/pkg/cmd/boot"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/namespace"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/jenkins-x/jx/pkg/versionstream/versionstreamrepo"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunOptions contains the command line arguments for this command
type RunOptions struct {
	boot.BootOptions
	KindResolver factory.KindResolver
	Gitter       gits.Gitter
	ChartName    string
	GitUserName  string
	GitToken     string
	BatchMode    bool
	JobMode      bool
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

const (
	defaultChartName = "jx-labs/jxl-boot"
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
	command.Flags().StringVarP(&options.GitUserName, "git-user", "", "", "specify the git user name to clone the development git repository. If not specified it is found from the secrets at $JX_SECRETS_YAML")
	command.Flags().StringVarP(&options.GitToken, "git-token", "", "", "specify the git token to clone the development git repository. If not specified it is found from the secrets at $JX_SECRETS_YAML")
	command.Flags().StringVarP(&options.GitRef, "git-ref", "", "master", "override the Git ref for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-url as well")
	command.Flags().StringVarP(&options.ChartName, "chart", "c", defaultChartName, "the chart name to use to install the boot Job")
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
	o.KindResolver.Dir = o.Dir
	if (o.JobMode || !clienthelpers.IsInCluster()) && os.Getenv("JX_DEBUG_JOB") != "true" {
		return o.RunBootJob()
	}
	bo := &o.BootOptions
	if bo.CommonOptions == nil {
		f := clients.NewFactory()
		bo.CommonOptions = opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
		bo.BatchMode = o.BatchMode
	}
	err := o.addUserPasswordForPrivateGitClone(true)
	if err != nil {
		return err
	}
	err = o.verifySecretsYAML()
	if err != nil {
		return err
	}
	return bo.Run()
}

// RunBootJob runs the boot installer Job
func (o *RunOptions) RunBootJob() error {
	err := o.detectGitURL()
	if err != nil {
		return err
	}
	requirements, gitURL, err := reqhelpers.FindRequirementsAndGitURL(o.KindResolver.GetFactory(), o.GitURL, o.Git(), o.Dir)
	if err != nil {
		return err
	}
	if gitURL == "" {
		return util.MissingOption("git-url")
	}

	err = o.addUserPasswordForPrivateGitClone(false)
	if err != nil {
		return errors.Wrapf(err, "could not default the git user and token to clone the git URL")
	}

	clusterName := requirements.Cluster.ClusterName
	log.Logger().Infof("running helmboot Job for cluster %s with git URL %s", util.ColorInfo(clusterName), util.ColorInfo(gitURL))

	log.Logger().Debug("deleting the old jx-boot chart ...")
	c := util.Command{
		Name: "helm",
		Args: []string{"delete", "jx-boot"},
	}
	_, err = c.RunWithoutRetry()
	if err != nil {
		log.Logger().Debugf("failed to delete the old jx-boot chart: %s", err.Error())
	}

	err = o.verifyBootSecret(requirements)
	if err != nil {
		return err
	}

	// lets add helm repository for jx-labs
	h := helmer.NewHelmCLI(o.Dir)
	_, err = helmer.AddHelmRepoIfMissing(h, helmer.LabsChartRepository, "jx-labs", "", "")
	if err != nil {
		return errors.Wrap(err, "failed to add Jenkins X Labs chart repository")
	}
	log.Logger().Infof("updating helm repositories")
	err = h.UpdateRepo()
	if err != nil {
		log.Logger().Warnf("failed to update helm repositories: %s", err.Error())
	}

	version, err := o.findChartVersion(requirements)
	if err != nil {
		return err
	}

	c = reqhelpers.GetBootJobCommand(requirements, gitURL, o.ChartName, version)

	commandLine := fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))

	log.Logger().Infof("running the command:\n\n%s\n\n", util.ColorInfo(commandLine))

	_, err = c.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to run command %s", commandLine)
	}

	return o.tailJobLogs()
}

func (o *RunOptions) tailJobLogs() error {
	a := jxadapt.NewJXAdapter(o.KindResolver.GetFactory(), o.Git(), o.BatchMode)
	client, ns, err := o.KindResolver.GetFactory().CreateKubeClient()
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

// Git lazily create a gitter if its not specified
func (o *RunOptions) Git() gits.Gitter {
	if o.Gitter == nil {
		o.Gitter = gits.NewGitCLI()
	}
	return o.Gitter
}

func (o *RunOptions) verifyBootSecret(requirements *config.RequirementsConfig) error {
	if requirements.SecretStorage == config.SecretStorageTypeVault {
		return nil
	}
	_, ns, err := o.KindResolver.GetFactory().CreateKubeClient()
	if err != nil {
		return errors.Wrap(err, "failed to create kube client")
	}

	reqNs := requirements.Cluster.Namespace
	if reqNs != "" && reqNs != ns {
		log.Logger().Infof("switching to the deployment namespace %s as we currently are in the %s namespace", util.ColorInfo(reqNs), util.ColorInfo(ns))

		f := clients.NewFactory()
		no := &namespace.NamespaceOptions{}
		no.CommonOptions = opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
		no.BatchMode = o.BatchMode
		no.Args = []string{reqNs}
		err = no.Run()
		if err != nil {
			return errors.Wrapf(err, "failed to switch to namespace %s", reqNs)
		}
		log.Logger().Infof("switched to the %s namespace", reqNs)
		ns = reqNs
	}

	o.KindResolver.Requirements = requirements
	sm, err := o.KindResolver.CreateSecretManager("")
	if err != nil {
		return errors.Wrap(err, "failed to create Secrets manager")
	}

	secretYaml := ""
	err = sm.UpsertSecrets(func(s string) (string, error) {
		secretYaml = s
		return s, nil
	}, "")
	if err != nil {
		return errors.Wrap(err, "failed to load Secrets YAML")
	}
	if secretYaml == "" {
		return fmt.Errorf("no secrets YAML found. Please run 'jxl boot secrets edit' to populate them")
	}
	err = secretmgr.VerifyBootSecrets(secretYaml)
	if err != nil {
		return errors.Wrapf(err, "invalid secrets yaml looking in namespace %s. Please run 'jxl boot secrets edit' to populate them", ns)
	}
	return nil
}

func (o *RunOptions) findChartVersion(req *config.RequirementsConfig) (string, error) {
	if o.ChartName == "" || o.ChartName[0] == '.' || o.ChartName[0] == '/' || o.ChartName[0] == '\\' || strings.Count(o.ChartName, "/") > 1 {
		// relative chart folder so ignore version
		return "", nil
	}

	f := clients.NewFactory()
	co := opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
	co.BatchMode = o.BatchMode

	u := req.VersionStream.URL
	ref := req.VersionStream.Ref
	version, err := getVersionNumber(versionstream.KindChart, o.ChartName, u, ref, o.Git(), co.GetIOFileHandles())
	if err != nil {
		return version, errors.Wrapf(err, "failed to find version of chart %s in version stream %s ref %s", o.ChartName, u, ref)
	}
	return version, nil
}

// getVersionNumber returns the version number for the given kind and name or blank string if there is no locked version
func getVersionNumber(kind versionstream.VersionKind, name, repo, gitRef string, git gits.Gitter, handles util.IOFileHandles) (string, error) {
	versioner, err := createVersionResolver(repo, gitRef, git, handles)
	if err != nil {
		return "", err
	}
	return versioner.StableVersionNumber(kind, name)
}

// createVersionResolver creates a new VersionResolver service
func createVersionResolver(versionRepository string, versionRef string, git gits.Gitter, handles util.IOFileHandles) (*versionstream.VersionResolver, error) {
	versionsDir, _, err := versionstreamrepo.CloneJXVersionsRepo(versionRepository, versionRef, nil, git, true, false, handles)
	if err != nil {
		return nil, err
	}
	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}, nil
}

func (o *RunOptions) verifySecretsYAML() error {
	yamlFile := os.Getenv("JX_SECRETS_YAML")
	if yamlFile == "" {
		return errors.Errorf("no $JX_SECRETS_YAML defined")
	}

	exists, err := util.FileExists(yamlFile)
	if err != nil {
		return errors.Wrapf(err, "failed to verify secrets YAML file exists: %s", yamlFile)
	}
	eo := &secrets.ExportOptions{
		KindResolver: o.KindResolver,
		OutFile:      yamlFile,
	}
	if !exists {
		// lets export the secrets to the yaml file
		err = eo.Run()
		if err != nil {
			return errors.Wrapf(err, "failed to generate the secrets YAML file: %s", yamlFile)
		}

		exists, err = util.FileExists(yamlFile)
		if err != nil {
			return errors.Wrapf(err, "failed to verify secrets YAML file exists after it was generated: %s", yamlFile)
		}
		if !exists {
			return errors.Errorf("no secrets YAML file exists after it was generated: %s", yamlFile)
		}
	}

	err = eo.VerifySecrets()
	if err != nil {
		return errors.Wrapf(err, "failed to verify the secrets for file %s", yamlFile)
	}
	return nil
}

func (o *RunOptions) addUserPasswordForPrivateGitClone(inCluster bool) error {
	err := o.detectGitURL()
	if err != nil {
		return err
	}
	u, err := url.Parse(o.GitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL %s", o.GitURL)
	}

	// lets check if we've already got a user and password
	if u.User != nil {
		user := u.User
		pwd, f := user.Password()
		if user.Username() != "" && pwd != "" && f {
			return nil
		}
	}

	username := o.GitUserName
	token := o.GitToken
	if username == "" || token == "" {
		if !inCluster {
			if username == "" {
				return util.MissingOption("git-user")
			}
			return util.MissingOption("git-token")
		}
		yamlFile := os.Getenv("JX_SECRETS_YAML")
		if yamlFile == "" {
			return errors.Errorf("no $JX_SECRETS_YAML defined")
		}
		data, err := ioutil.ReadFile(yamlFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load secrets YAML %s", yamlFile)
		}

		message := fmt.Sprintf("secrets YAML %s", yamlFile)
		username, token, err = secretmgr.PipelineUserTokenFromSecretsYAML(data, message)
		if err != nil {
			return err
		}
	}
	u.User = url.UserPassword(username, token)
	o.GitURL = u.String()
	o.KindResolver.GitURL = o.GitURL
	return nil
}

func (o *RunOptions) detectGitURL() error {
	if o.GitURL == "" {
		// lets try load the git URL from the secret
		gitURL, err := o.KindResolver.LoadBootRunGitURLFromSecret()
		if err != nil {
			return errors.Wrapf(err, "failed to load the boot git URL from the Secret")
		}
		if gitURL == "" {
			log.Logger().Warnf("no git-url specified and no boot git URL Secret found")
		}
		o.GitURL = gitURL
	}
	o.KindResolver.GitURL = o.GitURL
	return nil
}
