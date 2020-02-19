package reqhelpers

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/util"
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
