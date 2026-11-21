package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, args []string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, "kubectl", args...).Output()
}

type AppStatus struct {
	Name      string `json:"name"`
	Ready     int    `json:"ready"`
	Replicas  int    `json:"replicas"`
	Available bool   `json:"available"`
}

func ListDeployments(ctx context.Context, runner Runner, namespace, kubeContext string) ([]AppStatus, error) {
	args := []string{"get", "deployments", "-n", namespace, "-o", "json"}
	args = withContext(args, kubeContext)
	data, err := runner.Run(ctx, args)
	if err != nil {
		return nil, kubectlFailure(err)
	}
	return parseDeployments(data)
}

func RestartDeployment(ctx context.Context, runner Runner, namespace, kubeContext, name string) (string, error) {
	args := []string{"rollout", "restart", "deployment/" + name, "-n", namespace}
	args = withContext(args, kubeContext)
	output, err := runner.Run(ctx, args)
	if err != nil {
		return "", kubectlFailure(err)
	}
	return string(output), nil
}

func withContext(args []string, kubeContext string) []string {
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	return args
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

func parseDeployments(data []byte) ([]AppStatus, error) {
	var list deploymentList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("décodage de la sortie de kubectl : %w", err)
	}
	apps := make([]AppStatus, 0, len(list.Items))
	for _, item := range list.Items {
		replicas := 1
		if item.Spec.Replicas != nil {
			replicas = *item.Spec.Replicas
		}
		available := false
		for _, condition := range item.Status.Conditions {
			if condition.Type == "Available" && condition.Status == "True" {
				available = true
			}
		}
		apps = append(apps, AppStatus{
			Name:      item.Metadata.Name,
			Ready:     item.Status.ReadyReplicas,
			Replicas:  replicas,
			Available: available,
		})
	}
	return apps, nil
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
