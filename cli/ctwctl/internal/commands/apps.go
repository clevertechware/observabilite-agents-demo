package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	"github.com/urfave/cli/v3"
)

func appsCommand(deps dependencies) *cli.Command {
	return &cli.Command{
		Name:  "apps",
		Usage: "applications deployed on Kubernetes",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list the Deployments of the namespace",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listApps(ctx, cmd, app.NewApps(deps.newAppClient(settingsFrom(ctx))))
				},
			},
			{
				Name:      "restart",
				Usage:     "restart a Deployment (kubectl rollout restart)",
				ArgsUsage: "<name>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return restartApp(ctx, cmd, app.NewApps(deps.newAppClient(settingsFrom(ctx))))
				},
			},
		},
	}
}

func listApps(ctx context.Context, cmd *cli.Command, apps *app.Apps) error {
	statuses, err := apps.List(ctx)
	if err != nil {
		return err
	}

	if settingsFrom(ctx).Output == outputJSON {
		return writeJSON(writer(cmd), statuses)
	}

	tw := tabwriter.NewWriter(writer(cmd), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tREADY\tAVAILABLE")

	for _, status := range statuses {
		available := "no"
		if status.Available {
			available = "yes"
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%s\n", status.Name, status.Ready, status.Replicas, available)
	}

	return tw.Flush()
}

func restartApp(ctx context.Context, cmd *cli.Command, apps *app.Apps) error {
	if cmd.Args().Len() != 1 {
		return errors.New("a Deployment name is expected: ctwctl apps restart <name>")
	}

	name := cmd.Args().First()
	out, err := apps.Restart(ctx, name)
	if errors.Is(err, app.ErrInvalidName) {
		return fmt.Errorf("invalid Deployment name: %q", name)
	}
	if err != nil {
		return err
	}

	if settingsFrom(ctx).Output == outputJSON {
		return writeJSON(writer(cmd), map[string]any{"name": name, "restarted": true})
	}

	_, err = fmt.Fprintln(writer(cmd), strings.TrimSpace(out))
	return err
}
