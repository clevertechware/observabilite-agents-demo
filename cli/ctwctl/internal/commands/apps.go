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
		Usage: "applications déployées sur Kubernetes",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "liste les Deployments du namespace",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listApps(ctx, cmd, app.NewApps(deps.newAppClient(settingsFrom(ctx))))
				},
			},
			{
				Name:      "restart",
				Usage:     "redémarre un Deployment (kubectl rollout restart)",
				ArgsUsage: "<nom>",
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
	fmt.Fprintln(tw, "NOM\tPRÊTS\tDISPONIBLE")
	for _, status := range statuses {
		available := "non"
		if status.Available {
			available = "oui"
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%s\n", status.Name, status.Ready, status.Replicas, available)
	}
	return tw.Flush()
}

func restartApp(ctx context.Context, cmd *cli.Command, apps *app.Apps) error {
	if cmd.Args().Len() != 1 {
		return errors.New("un nom de Deployment est attendu : ctwctl apps restart <nom>")
	}
	name := cmd.Args().First()
	out, err := apps.Restart(ctx, name)
	if errors.Is(err, app.ErrInvalidName) {
		return fmt.Errorf("nom de Deployment invalide : %q", name)
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
