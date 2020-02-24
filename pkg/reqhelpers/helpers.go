package reqhelpers

import (
	"fmt"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

// GetDevEnvironmentConfig returns the dev environment for the given requirements or nil
func GetDevEnvironmentConfig(requirements *config.RequirementsConfig) *config.EnvironmentConfig {
	for i, e := range requirements.Environments {
		if e.Key == "dev" {
			return &requirements.Environments[i]
		}
	}
	return nil
}

// GetBootJobCommand returns the boot job command
func GetBootJobCommand(requirements *config.RequirementsConfig, gitURL string) util.Command {
	args := []string{"install", "jx-boot"}

	clusterName := requirements.Cluster.ClusterName
	if clusterName != "" {
		args = append(args, "--set", fmt.Sprintf("boot.clusterName=%s", clusterName))
	}

	if gitURL != "" {
		args = append(args, "--set", fmt.Sprintf("boot.bootGitURL=%s", gitURL))
	}
	// TODO detect local secret....
	args = append(args, "--set", "secrets.gsm.enabled=true")

	args = append(args, ".")

	return util.Command{
		Name: "helm",
		Args: args,
	}
}

// GetRequirementsFromEnvironment tries to find the development environment then the requirements from it
func GetRequirementsFromEnvironment(kubeClient kubernetes.Interface, jxClient versioned.Interface, namespace string) (*v1.Environment, *config.RequirementsConfig, error) {
	ns, _, err := kube.GetDevNamespace(kubeClient, namespace)
	if err != nil {
		log.Logger().Warnf("could not find the dev namespace from namespace %s due to %s", namespace, err.Error())
		ns = namespace
	}
	devEnv, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil {
		log.Logger().Warnf("could not find dev Environment in namespace %s", ns)
	}
	if devEnv != nil {
		requirements, err := config.GetRequirementsConfigFromTeamSettings(&devEnv.Spec.TeamSettings)
		if err != nil {
			return devEnv, nil, errors.Wrapf(err, "failed to load requirements from dev environment %s in namespace %s", devEnv.Name, ns)
		}
		if requirements != nil {
			return devEnv, requirements, nil
		}
	}

	// no dev environment found so lets return an empty environment
	if devEnv == nil {
		devEnv = kube.CreateDefaultDevEnvironment(ns)
	}
	if devEnv.Namespace == "" {
		devEnv.Namespace = ns
	}
	requirements := config.NewRequirementsConfig()
	return devEnv, requirements, nil
}
