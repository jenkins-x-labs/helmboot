package step

import (
	"context"
	"fmt"
	"strings"

	"github.com/jenkins-x-labs/helmboot/pkg/envfactory"
	"github.com/jenkins-x-labs/helmboot/pkg/jxadapt"
	"github.com/jenkins-x-labs/helmboot/pkg/reqhelpers"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	statusLong = templates.LongDesc(`
		Updates the git deployment status after a promotion
`)

	statusExample = templates.Examples(`
		# update the status in git after a promote pipeline
		helmboot step status
	`)
)

// StatusOptions the options for viewing running PRs
type StatusOptions struct {
	JXFactory jxfactory.Factory
}

// NewCmdStatus creates a command object for the "create" command
func NewCmdStatus() (*cobra.Command, *StatusOptions) {
	o := &StatusOptions{}

	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Updates the git deployment status after a promotion",
		Long:    statusLong,
		Example: statusExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	return cmd, o
}

// Run implements the command
func (o *StatusOptions) Run() error {
	if o.JXFactory == nil {
		o.JXFactory = jxfactory.NewFactory()
	}
	jxClient, ns, err := o.JXFactory.CreateJXClient()
	if err != nil {
		return err
	}
	kubeClient, _, err := o.JXFactory.CreateKubeClient()
	if err != nil {
		return err
	}

	list, err := jxClient.JenkinsV1().Releases(ns).List(metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Logger().Infof("no Releases found")
			return nil
		}
		return err
	}
	for _, r := range list.Items {
		err = o.updateStatus(&r, kubeClient, jxClient, ns)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *StatusOptions) updateStatus(r *v1.Release, kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string) error {
	owner := r.Spec.GitOwner
	gitURL := r.Spec.GitCloneURL
	if gitURL == "" {
		log.Logger().Warnf("no GitCloneURL for release %s", r.Name)
		return nil
	}
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL for release %s", r.Name)
	}
	server := gitInfo.HostURL()
	gitKind := gits.SaasGitKind(server)

	scmClient, _, err := o.JXAdapter().ScmClient(server, owner, gitKind)
	if err != nil {
		return errors.Wrapf(err, "failed to create SCM client for server %s", server)
	}

	if scmClient.Deployments == nil {
		log.Logger().Warnf("cannot update deployment status of release %s as the git server %s does not support Deployments", r.Name, server)
		return nil
	}
	ctx := context.Background()
	fullName := scm.Join(owner, r.Spec.GitRepository)

	devEnv, requirements, err := reqhelpers.GetRequirementsFromEnvironment(kubeClient, jxClient, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to get requirements from namespace %s", ns)
	}

	releaseNS := r.Namespace
	if releaseNS == "" {
		releaseNS = ns
	}
	environment := environmentLabel(jxClient, devEnv.Namespace, releaseNS)
	version := r.Spec.Version

	// lets try find the existing deployment if it exists
	deployments, _, err := scmClient.Deployments.List(ctx, fullName, scm.ListOptions{})
	if err != nil && !envfactory.IsScmNotFound(err) {
		return err
	}
	var deployment *scm.Deployment
	for _, d := range deployments {
		if d.Ref == version && d.Environment == environment {
			log.Logger().Infof("found existing deployment %s", d.Link)
			deployment = d
			break
		}
	}

	appName := r.Spec.GitRepository
	if deployment == nil {
		deploymentInput := &scm.DeploymentInput{
			Ref:                   version,
			Task:                  "deploy",
			Environment:           environment,
			Description:           fmt.Sprintf("release %s for version", appName, version),
			RequiredContexts:      nil,
			AutoMerge:             false,
			TransientEnvironment:  false,
			ProductionEnvironment: strings.HasPrefix(strings.ToLower(environment), "prod"),
		}
		deployment, _, err = scmClient.Deployments.Create(ctx, fullName, deploymentInput)
		if err != nil {
			return errors.Wrapf(err, "failed to create Deployment for server %s and release %s", server, r.Name)
		}
		log.Logger().Infof("created Deployment for release %s at %s", r.Name, deployment.Link)
	}

	// lets create a new status
	targetLink, err := services.FindServiceURL(kubeClient, ns, appName)
	if err != nil {
		log.Logger().Warnf("failed to find Target URL for app %s version %s: %s", appName, version, err.Error())
	}
	logLink, err := getLogURL(requirements, ns, appName)
	if err != nil {
		log.Logger().Warnf("failed to find Logs URL for app %s version %s: %s", appName, version, err.Error())
	}
	description := fmt.Sprintf("Deployment %s", strings.TrimPrefix(version, "v"))

	// TODO this could link to the UI?
	environmentLink := ""

	deploymentStatusInput := &scm.DeploymentStatusInput{
		State:           "success",
		TargetLink:      targetLink,
		LogLink:         logLink,
		Description:     description,
		Environment:     environment,
		EnvironmentLink: environmentLink,
		AutoInactive:    false,
	}
	status, _, err := scmClient.Deployments.CreateStatus(ctx, fullName, deployment.ID, deploymentStatusInput)
	if err != nil {
		return errors.Wrapf(err, "failed to create DeploymentStatus for server %s and release %s", server, r.Name)
	}
	log.Logger().Infof("created DeploymentStatus for release %s at %s with Logs URL %s and Target URL %s", r.Name, status.ID, logLink, targetLink)
	return nil
}

// environmentLabel returns the environment label for the given target namespace
func environmentLabel(jxClient versioned.Interface, devNS string, targetNS string) string {
	list, err := jxClient.JenkinsV1().Environments(devNS).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		log.Logger().Warnf("failed to find Environment CRDs in namespace %s", devNS)
	}
	if list != nil {
		for _, e := range list.Items {
			if e.Spec.Namespace == targetNS {
				answer := e.Spec.Label
				if answer == "" {
					answer = e.Name
				}
				return answer
			}
		}
	}
	// use a default value of the namespace without a prefix
	return strings.TrimPrefix(targetNS, "jx-")
}

func getLogURL(requirements *config.RequirementsConfig, ns string, appName string) (string, error) {
	c := &requirements.Cluster
	if c.Provider == cloud.GKE {
		return ContainerLogsURL(c.ProjectID, c.ClusterName, appName, ns), nil
	}
	return "", nil
}

// ContainerLogsURL generates the URL for a container logs URL
func ContainerLogsURL(projectName, clusterName, containerName, ns string) string {
	if projectName != "" && clusterName != "" && containerName != "" {
		return `https://console.cloud.google.com/logs/viewer?authuser=1&project=` + projectName + `&minLogLevel=0&expandAll=false&customFacets=&limitCustomFacetWidth=true&interval=PT1H&resource=k8s_container%2Fcluster_name%2F` + clusterName + `%2Fnamespace_name%2F` + ns + `%2Fcontainer_name%2F` + containerName + `&dateRangeUnbound=both`
	}
	return ""
}

// JXAdapter creates an adapter to the jx code
func (o *StatusOptions) JXAdapter() *jxadapt.JXAdapter {
	return jxadapt.NewJXAdapter(o.JXFactory, gits.NewGitCLI(), true)
}
