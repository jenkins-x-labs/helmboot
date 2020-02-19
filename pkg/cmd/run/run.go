package run

import (
	"os"

	"github.com/jenkins-x-labs/helmboot/pkg/common"
	"github.com/jenkins-x/jx/pkg/cmd/boot"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

// HelmBootOptions contains the command line arguments for this command
type HelmBootOptions struct {
	boot.BootOptions
	Batch bool
}

var (
	stepCustomPipelineLong = templates.LongDesc(`
		This command boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps

`)

	stepCustomPipelineExample = templates.Examples(`
		# triggers the Jenkinsfile in the current directory in a Jenkins server installed via the Jenkins Operator
		tp
`)
)

// NewHelmBootRun creates the new command
func NewHelmBootRun() *cobra.Command {
	options := HelmBootOptions{}
	command := &cobra.Command{
		Use:     "run",
		Short:   "boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps. This is usually ran from inside the cluster",
		Long:    stepCustomPipelineLong,
		Example: stepCustomPipelineExample,
		Run: func(command *cobra.Command, args []string) {
			common.SetLoggingLevel(command, args)
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the Jenkins X Pipeline, requirements and charts")
	command.Flags().StringVarP(&options.GitURL, "git-url", "u", common.DefaultBootHelmfileRepository, "override the Git clone URL for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-ref as well")
	command.Flags().StringVarP(&options.GitRef, "git-ref", "", "master", "override the Git ref for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-url as well")
	command.Flags().StringVarP(&options.VersionStreamURL, "versions-repo", "", common.DefaultVersionsURL, "the bootstrap URL for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	command.Flags().StringVarP(&options.VersionStreamRef, "versions-ref", "", common.DefaultVersionsRef, "the bootstrap ref for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	command.Flags().StringVarP(&options.HelmLogLevel, "helm-log", "v", "", "sets the helm logging level from 0 to 9. Passed into the helm CLI via the '-v' argument. Useful to diagnose helm related issues")
	command.Flags().StringVarP(&options.RequirementsFile, "requirements", "r", "", "requirements file which will overwrite the default requirements file")

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	command.PersistentFlags().BoolVarP(&options.Batch, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")
	return command
}

// Run implements the command
func (o *HelmBootOptions) Run() error {
	bo := &o.BootOptions
	if bo.CommonOptions == nil {
		f := clients.NewFactory()
		bo.CommonOptions = opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
		bo.BatchMode = o.Batch
	}
	return bo.Run()
}
