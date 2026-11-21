package commands

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	"github.com/urfave/cli/v3"
)

var deploymentName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

type appClient interface {
	ListApps(ctx context.Context) ([]app.Status, error)
	RestartApp(ctx context.Context, name string) (string, error)
}

func appsCommand(deps dependencies) *cli.Command {
	return &cli.Command{
		Name:  "apps",
		Usage: "applications déployées sur Kubernetes",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "liste les Deployments du namespace",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listApps(ctx, cmd, deps.newAppClient(settingsFrom(ctx)))
				},
			},
			{
				Name:      "restart",
				Usage:     "redémarre un Deployment (kubectl rollout restart)",
				ArgsUsage: "<nom>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return restartApp(ctx, cmd, deps.newAppClient(settingsFrom(ctx)))
				},
			},
		},
	}
}

func listApps(ctx context.Context, cmd *cli.Command, client appClient) error {
	apps, err := client.ListApps(ctx)
	if err != nil {
		return err
	}
	if settingsFrom(ctx).Output == outputJSON {
		return writeJSON(writer(cmd), apps)
	}
	tw := tabwriter.NewWriter(writer(cmd), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NOM\tPRÊTS\tDISPONIBLE")
	for _, status := range apps {
		available := "non"
		if status.Available {
			available = "oui"
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%s\n", status.Name, status.Ready, status.Replicas, available)
	}
	return tw.Flush()
}

func restartApp(ctx context.Context, cmd *cli.Command, client appClient) error {
	if cmd.Args().Len() != 1 {
		return errors.New("un nom de Deployment est attendu : ctwctl apps restart <nom>")
	}
	name := cmd.Args().First()
	if !deploymentName.MatchString(name) {
		return fmt.Errorf("nom de Deployment invalide : %q", name)
	}
	out, err := client.RestartApp(ctx, name)
	if err != nil {
		return err
	}
	if settingsFrom(ctx).Output == outputJSON {
		return writeJSON(writer(cmd), map[string]any{"name": name, "restarted": true})
	}
	_, err = fmt.Fprintln(writer(cmd), strings.TrimSpace(out))
	return err
}
