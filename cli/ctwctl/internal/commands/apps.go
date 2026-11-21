package commands

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/kube"
	"github.com/urfave/cli/v3"
)

var deploymentName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

func appsCommand(deps dependencies) *cli.Command {
	return &cli.Command{
		Name:  "apps",
		Usage: "applications déployées sur Kubernetes",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "liste les Deployments du namespace",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listApps(ctx, cmd, deps.kubectl)
				},
			},
			{
				Name:      "restart",
				Usage:     "redémarre un Deployment (kubectl rollout restart)",
				ArgsUsage: "<nom>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return restartApp(ctx, cmd, deps.kubectl)
				},
			},
		},
	}
}

func listApps(ctx context.Context, cmd *cli.Command, runner kube.Runner) error {
	s := settingsFrom(ctx)
	apps, err := kube.ListDeployments(ctx, runner, s.Kube.Namespace, s.Kube.Context)
	if err != nil {
		return err
	}
	if s.Output == outputJSON {
		return writeJSON(writer(cmd), apps)
	}
	tw := tabwriter.NewWriter(writer(cmd), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NOM\tPRÊTS\tDISPONIBLE")
	for _, app := range apps {
		available := "non"
		if app.Available {
			available = "oui"
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%s\n", app.Name, app.Ready, app.Replicas, available)
	}
	return tw.Flush()
}

func restartApp(ctx context.Context, cmd *cli.Command, runner kube.Runner) error {
	if cmd.Args().Len() != 1 {
		return errors.New("un nom de Deployment est attendu : ctwctl apps restart <nom>")
	}
	name := cmd.Args().First()
	if !deploymentName.MatchString(name) {
		return fmt.Errorf("nom de Deployment invalide : %q", name)
	}
	s := settingsFrom(ctx)
	out, err := kube.RestartDeployment(ctx, runner, s.Kube.Namespace, s.Kube.Context, name)
	if err != nil {
		return err
	}
	if s.Output == outputJSON {
		return writeJSON(writer(cmd), map[string]any{"name": name, "restarted": true})
	}
	_, err = fmt.Fprintln(writer(cmd), strings.TrimSpace(string(out)))
	return err
}
