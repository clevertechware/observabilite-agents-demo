package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
)

type kubectlRunner interface {
	Run(ctx context.Context, args []string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, "kubectl", args...).Output()
}

var deploymentName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

type appStatus struct {
	Name      string `json:"name"`
	Ready     int    `json:"ready"`
	Replicas  int    `json:"replicas"`
	Available bool   `json:"available"`
}

type deploymentList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Replicas *int `json:"replicas"`
		} `json:"spec"`
		Status struct {
			ReadyReplicas int `json:"readyReplicas"`
			Conditions    []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
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

func kubectlArgs(s settings, args ...string) []string {
	args = append(args, "-n", s.Kube.Namespace)
	return args
}

func withContext(s settings, args []string) []string {
	if s.Kube.Context != "" {
		args = append(args, "--context", s.Kube.Context)
	}
	return args
}

func listApps(ctx context.Context, cmd *cli.Command, runner kubectlRunner) error {
	s := settingsFrom(ctx)
	args := withContext(s, append(kubectlArgs(s, "get", "deployments"), "-o", "json"))
	out, err := runner.Run(ctx, args)
	if err != nil {
		return kubectlFailure(err)
	}
	apps, err := parseDeployments(out)
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

func parseDeployments(data []byte) ([]appStatus, error) {
	var list deploymentList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("décodage de la sortie de kubectl : %w", err)
	}
	apps := make([]appStatus, 0, len(list.Items))
	for _, item := range list.Items {
		replicas := 1
		if item.Spec.Replicas != nil {
			replicas = *item.Spec.Replicas
		}
		available := false
		for _, c := range item.Status.Conditions {
			if c.Type == "Available" && c.Status == "True" {
				available = true
			}
		}
		apps = append(apps, appStatus{
			Name:      item.Metadata.Name,
			Ready:     item.Status.ReadyReplicas,
			Replicas:  replicas,
			Available: available,
		})
	}
	return apps, nil
}

func restartApp(ctx context.Context, cmd *cli.Command, runner kubectlRunner) error {
	if cmd.Args().Len() != 1 {
		return errors.New("un nom de Deployment est attendu : ctwctl apps restart <nom>")
	}
	name := cmd.Args().First()
	if !deploymentName.MatchString(name) {
		return fmt.Errorf("nom de Deployment invalide : %q", name)
	}
	s := settingsFrom(ctx)
	args := withContext(s, kubectlArgs(s, "rollout", "restart", "deployment/"+name))
	out, err := runner.Run(ctx, args)
	if err != nil {
		return kubectlFailure(err)
	}
	if s.Output == outputJSON {
		return writeJSON(writer(cmd), map[string]any{"name": name, "restarted": true})
	}
	_, err = fmt.Fprintln(writer(cmd), strings.TrimSpace(string(out)))
	return err
}

func kubectlFailure(err error) error {
	var exitErr *exec.ExitError
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return errors.New("kubectl est introuvable dans le PATH")
	case errors.As(err, &exitErr) && len(exitErr.Stderr) > 0:
		return fmt.Errorf("kubectl : %s", strings.TrimSpace(string(exitErr.Stderr)))
	default:
		return fmt.Errorf("kubectl : %w", err)
	}
}
