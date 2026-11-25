// Package apps provides the Kubernetes application commands for ctwctl.
package apps

import (
	"context"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/kube"
	"github.com/urfave/cli/v3"
)

// Dependencies contains the configuration and the services used by the application commands.
type Dependencies struct {
	// LoadSettings resolves the settings of the invoked command. It is required.
	LoadSettings cmdutil.SettingsLoader
	// NewAppClient creates a Kubernetes client from the resolved configuration.
	NewAppClient func(config.Settings) app.Client
}

func (d Dependencies) withDefaults() Dependencies {
	if d.NewAppClient == nil {
		d.NewAppClient = func(s config.Settings) app.Client { return kube.NewClient(s.Kube.Namespace, s.Kube.Context) }
	}

	return d
}

// NewCommand creates the application command tree.
func NewCommand(deps Dependencies) *cli.Command {
	deps = deps.withDefaults()

	return &cli.Command{
		Name:  "apps",
		Usage: "applications deployed on Kubernetes",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list the Deployments of the namespace",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listApps(ctx, cmd, deps)
				},
			},
			{
				Name:      "restart",
				Usage:     "restart a Deployment (kubectl rollout restart)",
				ArgsUsage: "<name>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return restartApp(ctx, cmd, deps)
				},
			},
		},
	}
}
