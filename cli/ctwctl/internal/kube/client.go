package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
)

type Client struct {
	namespace string
	context   string
	run       func(ctx context.Context, args ...string) ([]byte, error)
}

func NewClient(namespace, kubeContext string) *Client {
	return &Client{namespace: namespace, context: kubeContext, run: runKubectl}
}

func runKubectl(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "kubectl", args...).Output()
}

func (c *Client) ListApps(ctx context.Context) ([]app.Status, error) {
	data, err := c.kubectl(ctx, "get", "deployments", "-n", c.namespace, "-o", "json")
	if err != nil {
		return nil, err
	}
	return parseDeployments(data)
}

func (c *Client) RestartApp(ctx context.Context, name string) (string, error) {
	output, err := c.kubectl(ctx, "rollout", "restart", "deployment/"+name, "-n", c.namespace)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (c *Client) kubectl(ctx context.Context, args ...string) ([]byte, error) {
	if c.context != "" {
		args = append(args, "--context", c.context)
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return nil, kubectlFailure(err)
	}

	return output, nil
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

func parseDeployments(data []byte) ([]app.Status, error) {
	var list deploymentList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("decode kubectl output: %w", err)
	}

	apps := make([]app.Status, 0, len(list.Items))
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

		apps = append(apps, app.Status{
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
		return errors.New("kubectl not found in PATH")
	case errors.As(err, &exitErr) && len(exitErr.Stderr) > 0:
		return fmt.Errorf("kubectl: %s", strings.TrimSpace(string(exitErr.Stderr)))
	default:
		return fmt.Errorf("kubectl: %w", err)
	}
}
