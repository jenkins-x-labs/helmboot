package show

import (
	"fmt"

	"github.com/jenkins-x-labs/helmboot/pkg/envfactory"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	showLong = templates.LongDesc(`
		Displays the boot command line
`)

	showExample = templates.Examples(`
		# display the command line to run the boot job
		helmboot show
	`)
)

// ShowOptions the options for viewing running PRs
type ShowOptions struct {
	envfactory.EnvFactory
}

// NewCmdShow creates a command object for the "show" command
func NewCmdShow() (*cobra.Command, *ShowOptions) {
	o := &ShowOptions{}

	cmd := &cobra.Command{
		Use:     "show",
		Short:   "Displays the command to boot",
		Long:    showLong,
		Example: showExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	return cmd, o
}

// Run implements the command
func (o *ShowOptions) Run() error {
	requirements, gitURL, err := o.findRequirementsAndGitURL()
	if err != nil {
		return err
	}
	if gitURL == "" {
		return fmt.Errorf("could not detect the gitURL for the current Dev Environment")
	}

	return o.PrintBootJobInstructions(requirements, gitURL)
}

func (o *ShowOptions) findRequirementsAndGitURL() (*config.RequirementsConfig, string, error) {
	if o.JXFactory == nil {
		o.JXFactory = jxfactory.NewFactory()
	}
	jxClient, ns, err := o.JXFactory.CreateJXClient()
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to create the Jenkins X client")
	}

	dev, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to find the 'dev' Environment in namespace %s", ns)
	}

	gitURL := dev.Spec.Source.URL

	requirements, err := config.GetRequirementsConfigFromTeamSettings(&dev.Spec.TeamSettings)
	if err != nil {
		return nil, gitURL, errors.Wrapf(err, "failed to find requirements in team settings for the 'dev' Environment for namespace %s", ns)
	}
	return requirements, gitURL, nil
}
