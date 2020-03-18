package reqhelpers

import (
	"fmt"
	"io/ioutil"
	"os"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// RequirementFlags for the boolean flags we only update if specified on the CLI
type RequirementFlags struct {
	Repository                                                      string
	IngressKind                                                     string
	AutoUpgrade, EnvironmentGitPublic, GitPublic, EnvironmentRemote bool
	GitOps, Kaniko, Terraform, ExternalDNS, TLS                     bool
	VaultRecreateBucket, VaultDisableURLDiscover                    bool
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
func GetBootJobCommand(requirements *config.RequirementsConfig, gitURL string, chartName string, version string) util.Command {
	args := []string{"install", "jx-boot"}

	clusterName := requirements.Cluster.ClusterName
	if clusterName != "" {
		args = append(args, "--set", fmt.Sprintf("boot.clusterName=%s", clusterName))
	}

	if gitURL != "" {
		args = append(args, "--set", fmt.Sprintf("boot.bootGitURL=%s", gitURL))
	}
	if version != "" {
		args = append(args, "--version", version)
	}
	args = append(args, chartName)

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

// OverrideRequirements allows CLI overrides
func OverrideRequirements(cmd *cobra.Command, args []string, dir string, outputRequirements *config.RequirementsConfig, flags *RequirementFlags) error {
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

// ValidateApps validates the apps match the requirements
func ValidateApps(dir string) (*config.AppConfig, string, error) {
	requirements, _, err := config.LoadRequirementsConfig(dir)
	if err != nil {
		return nil, "", err
	}
	apps, appsFileName, err := config.LoadAppConfig(dir)

	modified := false
	if requirements.Repository != config.RepositoryTypeNexus {
		if removeApp(apps, "jenkins-x/nexus") {
			modified = true
		}
	}
	if requirements.Repository == config.RepositoryTypeBucketRepo {
		if removeApp(apps, "jenkins-x/chartmuseum") {
			modified = true
		}
		if addApp(apps, "jenkins-x/bucketrepo", "repositories") {
			modified = true
		}
	}

	if requirements.Ingress.Kind == config.IngressTypeIstio {
		if removeApp(apps, "stable/nginx-ingress") {
			modified = true
		}
		if addApp(apps, "jx-labs/istio", "jenkins-x/jxboot-helmfile-resources") {
			modified = true
		}
	}

	if shouldHaveCertManager(requirements) {
		if addApp(apps, "jetstack/cert-manager", "jenkins-x/jxboot-helmfile-resources") {
			modified = true
		}
		if addApp(apps, "jenkins-x/acme", "jenkins-x/jxboot-helmfile-resources") {
			modified = true
		}
	}
	if shouldHaveExternalDNS(requirements) {
		if addApp(apps, "bitnami/externaldns", "jenkins-x/jxboot-helmfile-resources") {
			modified = true
		}
	}

	if modified {
		err = apps.SaveConfig(appsFileName)
		if err != nil {
			return apps, appsFileName, errors.Wrapf(err, "failed to save modified file %s", appsFileName)
		}
	}
	return apps, appsFileName, err
}

func shouldHaveCertManager(requirements *config.RequirementsConfig) bool {
	return requirements.Ingress.TLS.Enabled && requirements.Ingress.TLS.SecretName == ""
}

func shouldHaveExternalDNS(requirements *config.RequirementsConfig) bool {
	return requirements.Ingress.ExternalDNS
}

func addApp(apps *config.AppConfig, chartName string, beforeName string) bool {
	idx := -1
	for i, a := range apps.Apps {
		switch a.Name {
		case chartName:
			return false
		case beforeName:
			idx = i
		}
	}
	app := config.App{Name: chartName}

	// if we have a repositories chart lets add apps before that
	if idx >= 0 {
		newApps := append([]config.App{app}, apps.Apps[idx:]...)
		apps.Apps = append(apps.Apps[0:idx], newApps...)
	} else {
		apps.Apps = append(apps.Apps, app)
	}
	return true
}

func removeApp(apps *config.AppConfig, chartName string) bool {
	for i, a := range apps.Apps {
		if a.Name == chartName {
			apps.Apps = append(apps.Apps[0:i], apps.Apps[i+1:]...)
			return true
		}
	}
	return false
}

func applyDefaults(cmd *cobra.Command, r *config.RequirementsConfig, flags *RequirementFlags) error {
	// override boolean flags if specified
	if FlagChanged(cmd, "autoupgrade") {
		r.AutoUpdate.Enabled = flags.AutoUpgrade
	}
	if FlagChanged(cmd, "env-git-public") {
		r.Cluster.EnvironmentGitPublic = flags.EnvironmentGitPublic
	}
	if FlagChanged(cmd, "externaldns") {
		r.Ingress.ExternalDNS = flags.ExternalDNS
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
	if FlagChanged(cmd, "tls") {
		r.Ingress.TLS.Enabled = flags.TLS
	}

	if flags.Repository != "" {
		r.Repository = config.RepositoryType(flags.Repository)
	}
	if flags.IngressKind != "" {
		r.Ingress.Kind = config.IngressType(flags.IngressKind)
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
