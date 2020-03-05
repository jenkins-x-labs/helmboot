package reqhelpers

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// RequirementBools for the boolean flags we only update if specified on the CLI
type RequirementBools struct {
	AutoUpgrade, EnvironmentGitPublic, GitPublic, EnvironmentRemote, GitOps, Kaniko, Terraform bool
	VaultRecreateBucket, VaultDisableURLDiscover                                               bool
}

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

// GetRequirementsFromGit clones the given git repository to get the requirements
func GetRequirementsFromGit(gitURL string) (*config.RequirementsConfig, error) {
	tempDir, err := ioutil.TempDir("", "jx-boot-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	log.Logger().Debugf("cloning %s to %s", gitURL, tempDir)

	gitter := gits.NewGitCLI()
	err = gitter.Clone(gitURL, tempDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to git clone %s to dir %s", gitURL, tempDir)
	}

	requirements, _, err := config.LoadRequirementsConfig(tempDir)
	if err != nil {
		return requirements, errors.Wrapf(err, "failed to requirements YAML file from %s", tempDir)
	}
	return requirements, nil
}

// OverrideRequirements
func OverrideRequirements(cmd *cobra.Command, args []string, dir string, outputRequirements *config.RequirementsConfig, flags *RequirementBools) error {
	requirements, fileName, err := config.LoadRequirementsConfig(dir)
	if err != nil {
		return err
	}
	*outputRequirements = *requirements

	// lets re-parse the CLI arguments to re-populate the loaded requirements
	if len(args) == 0 {
		args = os.Args

		// lets trim the actual command which could be `helmboot create` or `jxl boot create` or `jx alpha boot create`
		for i := range args {
			if i == 0 {
				continue
			}
			if i > 3 {
				break
			}
			if args[i] == "create" {
				args = args[i+1:]
				break
			}
		}
	}

	err = cmd.Flags().Parse(args)
	if err != nil {
		return errors.Wrap(err, "failed to reparse arguments")
	}

	err = applyDefaults(cmd, outputRequirements, flags)
	if err != nil {
		return err
	}

	err = outputRequirements.SaveConfig(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to save %s", fileName)
	}

	log.Logger().Infof("saved file: %s", util.ColorInfo(fileName))
	return nil
}

func applyDefaults(cmd *cobra.Command, r *config.RequirementsConfig, flags *RequirementBools) error {
	// override boolean flags if specified
	if FlagChanged(cmd, "autoupgrade") {
		r.AutoUpdate.Enabled = flags.AutoUpgrade
	}
	if FlagChanged(cmd, "env-git-public") {
		r.Cluster.EnvironmentGitPublic = flags.EnvironmentGitPublic
	}
	if FlagChanged(cmd, "git-public") {
		r.Cluster.GitPublic = flags.GitPublic
	}
	if FlagChanged(cmd, "gitops") {
		r.GitOps = flags.GitOps
	}
	if FlagChanged(cmd, "kaniko") {
		r.Kaniko = flags.Kaniko
	}
	if FlagChanged(cmd, "terraform") {
		r.Terraform = flags.Terraform
	}
	if FlagChanged(cmd, "vault-disable-url-discover") {
		r.Vault.DisableURLDiscovery = flags.VaultDisableURLDiscover
	}
	if FlagChanged(cmd, "vault-recreate-bucket") {
		r.Vault.RecreateBucket = flags.VaultRecreateBucket
	}

	if flags.EnvironmentRemote {
		for i, e := range r.Environments {
			if e.Key == "dev" {
				continue
			}
			r.Environments[i].RemoteCluster = true
		}
	}

	gitKind := r.Cluster.GitKind
	gitKinds := append(gits.KindGits, "fake")
	if gitKind != "" && util.StringArrayIndex(gitKinds, gitKind) < 0 {
		return util.InvalidOption("git-kind", gitKind, gits.KindGits)
	}

	// default flags if associated values
	if r.AutoUpdate.Schedule != "" {
		r.AutoUpdate.Enabled = true
	}
	if r.Ingress.TLS.Email != "" {
		r.Ingress.TLS.Enabled = true
	}

	// enable storage if we specify a URL
	storage := &r.Storage
	if storage.Logs.URL != "" && storage.Reports.URL == "" {
		storage.Reports.URL = storage.Logs.URL
	}
	defaultStorage(&storage.Backup)
	defaultStorage(&storage.Logs)
	defaultStorage(&storage.Reports)
	defaultStorage(&storage.Repository)
	return nil
}

// FlagChanged returns true if the given flag was supplied on the command line
func FlagChanged(cmd *cobra.Command, name string) bool {
	if cmd != nil {
		f := cmd.Flag(name)
		if f != nil {
			return f.Changed
		}
	}
	return false
}

func defaultStorage(storage *config.StorageEntryConfig) {
	if storage.URL != "" {
		storage.Enabled = true
	}
}

// AddRequirementsFlagsOptions add CLI options to the flags
func AddRequirementsFlagsOptions(cmd *cobra.Command, flags *RequirementBools) {
	cmd.Flags().BoolVarP(&flags.AutoUpgrade, "autoupgrade", "", false, "enables or disables auto upgrades")
	cmd.Flags().BoolVarP(&flags.EnvironmentRemote, "env-remote", "", false, "if enables then all other environments than dev (staging & production by default) will be configured to be in remote clusters")
	cmd.Flags().BoolVarP(&flags.EnvironmentGitPublic, "env-git-public", "", false, "enables or disables whether the environment repositories should be public")
	cmd.Flags().BoolVarP(&flags.GitPublic, "git-public", "", false, "enables or disables whether the project repositories should be public")
	cmd.Flags().BoolVarP(&flags.GitOps, "gitops", "g", false, "enables or disables the use of gitops")
	cmd.Flags().BoolVarP(&flags.Kaniko, "kaniko", "", false, "enables or disables the use of kaniko")
	cmd.Flags().BoolVarP(&flags.Terraform, "terraform", "", false, "enables or disables the use of terraform")
	cmd.Flags().BoolVarP(&flags.VaultRecreateBucket, "vault-recreate-bucket", "", false, "enables or disables whether to rereate the secret bucket on boot")
	cmd.Flags().BoolVarP(&flags.VaultDisableURLDiscover, "vault-disable-url-discover", "", false, "override the default lookup of the Vault URL, could be incluster service or external ingress")
}

// AddRequirementsOptions add CLI flags to the requirements
func AddRequirementsOptions(cmd *cobra.Command, r *config.RequirementsConfig) {
	cmd.Flags().StringVarP(&r.BootConfigURL, "boot-config-url", "", "", "specify the boot configuration git URL")

	// auto upgrade
	cmd.Flags().StringVarP(&r.AutoUpdate.Schedule, "autoupdate-schedule", "", "", "the cron schedule for auto upgrading your cluster")

	// cluster
	cmd.Flags().StringVarP(&r.Cluster.ClusterName, "cluster", "c", "", "configures the cluster name")
	cmd.Flags().StringVarP(&r.Cluster.Namespace, "namespace", "n", "", "configures the namespace to use")
	cmd.Flags().StringVarP(&r.Cluster.Provider, "provider", "p", "", "configures the kubernetes provider.  Supported providers: "+cloud.KubernetesProviderOptions())
	cmd.Flags().StringVarP(&r.Cluster.ProjectID, "project", "", "", "configures the Google Project ID")
	cmd.Flags().StringVarP(&r.Cluster.Registry, "registry", "", "", "configures the host name of the container registry")
	cmd.Flags().StringVarP(&r.Cluster.Region, "region", "r", "", "configures the cloud region")
	cmd.Flags().StringVarP(&r.Cluster.Zone, "zone", "z", "", "configures the cloud zone")

	cmd.Flags().StringVarP(&r.Cluster.ExternalDNSSAName, "extdns-sa", "", "", "configures the External DNS service account name")
	cmd.Flags().StringVarP(&r.Cluster.KanikoSAName, "kaniko-sa", "", "", "configures the Kaniko service account name")

	AddGitRequirementsOptions(cmd, r)

	// ingress
	cmd.Flags().StringVarP(&r.Ingress.Domain, "domain", "d", "", "configures the domain name")
	cmd.Flags().StringVarP(&r.Ingress.TLS.Email, "tls-email", "", "", "the TLS email address to enable TLS on the domain")

	// storage
	cmd.Flags().StringVarP(&r.Storage.Logs.URL, "bucket-logs", "", "", "the bucket URL to store logs")
	cmd.Flags().StringVarP(&r.Storage.Backup.URL, "bucket-backups", "", "", "the bucket URL to store backups")
	cmd.Flags().StringVarP(&r.Storage.Repository.URL, "bucket-repo", "", "", "the bucket URL to store repository artifacts")
	cmd.Flags().StringVarP(&r.Storage.Reports.URL, "bucket-reports", "", "", "the bucket URL to store reports. If not specified default to te logs bucket")

	// vault
	cmd.Flags().StringVarP(&r.Vault.Name, "vault-name", "", "", "specify the vault name")
	cmd.Flags().StringVarP(&r.Vault.Bucket, "vault-bucket", "", "", "specify the vault bucket")
	cmd.Flags().StringVarP(&r.Vault.Keyring, "vault-keyring", "", "", "specify the vault key ring")
	cmd.Flags().StringVarP(&r.Vault.Key, "vault-key", "", "", "specify the vault key")
	cmd.Flags().StringVarP(&r.Vault.ServiceAccount, "vault-sa", "", "", "specify the vault Service Account name")

	// velero
	cmd.Flags().StringVarP(&r.Velero.ServiceAccount, "velero-sa", "", "", "specify the Velero Service Account name")
	cmd.Flags().StringVarP(&r.Velero.Namespace, "velero-ns", "", "", "specify the Velero Namespace")

	// version stream
	cmd.Flags().StringVarP(&r.VersionStream.URL, "version-stream-url", "", "", "specify the Version Stream git URL")
	cmd.Flags().StringVarP(&r.VersionStream.Ref, "version-stream-ref", "", "", "specify the Version Stream git reference (branch, tag, sha)")
}

// AddGitRequirementsOptions adds git specific overrides to the given requirements
func AddGitRequirementsOptions(cmd *cobra.Command, r *config.RequirementsConfig) {
	cmd.Flags().StringVarP(&r.Cluster.GitKind, "git-kind", "", "", fmt.Sprintf("the kind of git repository to use. Possible values: %s", strings.Join(gits.KindGits, ", ")))
	cmd.Flags().StringVarP(&r.Cluster.GitName, "git-name", "", "", "the name of the git repository")
	cmd.Flags().StringVarP(&r.Cluster.GitServer, "git-server", "", "", "the git server host such as https://github.com or https://gitlab.com")
	cmd.Flags().StringVarP(&r.Cluster.EnvironmentGitOwner, "env-git-owner", "", "", "the git owner (organisation or user) used to own the git repositories for the environments")
}
