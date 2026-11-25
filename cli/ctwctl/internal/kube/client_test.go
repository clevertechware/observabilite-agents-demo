package kube

import (
	"context"
	"os/exec"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var deploymentsJSON = []byte(`{
  "kind": "List",
  "items": [
    {"metadata": {"name": "order-service"}, "spec": {"replicas": 1},
     "status": {"readyReplicas": 1, "conditions": [{"type": "Progressing", "status": "True"}, {"type": "Available", "status": "True"}]}},
    {"metadata": {"name": "pricing-service"}, "spec": {"replicas": 2},
     "status": {"readyReplicas": 1, "conditions": [{"type": "Available", "status": "False"}]}},
    {"metadata": {"name": "load-generator"}, "spec": {}, "status": {}}
  ]
}`)

func fakeKubectl(t *testing.T, client *Client, wantArgs []string, output []byte, err error) {
	t.Helper()
	client.run = func(_ context.Context, args ...string) ([]byte, error) {
		if wantArgs != nil {
			assert.Equal(t, wantArgs, args)
		}
		return output, err
	}
}

func TestClientListApps_GetsDeploymentsOfTheNamespaceAsJSON(t *testing.T) {
	t.Parallel()

	client := NewClient("demo", "")
	fakeKubectl(t, client, []string{"get", "deployments", "-n", "demo", "-o", "json"}, []byte(`{"items":[]}`), nil)

	apps, err := client.ListApps(t.Context())

	require.NoError(t, err)
	assert.Empty(t, apps)
}

func TestClientListApps_WhenContextIsGiven_ForwardsItToKubectl(t *testing.T) {
	t.Parallel()

	client := NewClient("staging", "prod-cluster")
	fakeKubectl(t, client,
		[]string{"get", "deployments", "-n", "staging", "-o", "json", "--context", "prod-cluster"},
		[]byte(`{"items":[]}`), nil)

	_, err := client.ListApps(t.Context())

	require.NoError(t, err)
}

func TestClientListApps_ReportsReadinessReplicasAndAvailability(t *testing.T) {
	t.Parallel()

	client := NewClient("demo", "")
	fakeKubectl(t, client, nil, deploymentsJSON, nil)

	apps, err := client.ListApps(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []app.Status{
		{Name: "order-service", Ready: 1, Replicas: 1, Available: true},
		{Name: "pricing-service", Ready: 1, Replicas: 2, Available: false},
		{Name: "load-generator", Ready: 0, Replicas: 1, Available: false},
	}, apps)
}

func TestClientListApps_WhenKubectlPrintsInvalidJSON_ReturnsDecodeError(t *testing.T) {
	t.Parallel()

	client := NewClient("demo", "")
	fakeKubectl(t, client, nil, []byte("pas du json"), nil)

	_, err := client.ListApps(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestClientRestartApp_RunsRolloutRestartOnTheNamedDeployment(t *testing.T) {
	t.Parallel()

	client := NewClient("demo", "prod-cluster")
	fakeKubectl(t, client,
		[]string{"rollout", "restart", "deployment/load-generator", "-n", "demo", "--context", "prod-cluster"},
		[]byte("deployment.apps/load-generator restarted\n"), nil)

	out, err := client.RestartApp(t.Context(), "load-generator")

	require.NoError(t, err)
	assert.Equal(t, "deployment.apps/load-generator restarted\n", out)
}

func TestClient_WhenKubectlFails_TranslatesTheError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"stderr is reported", &exec.ExitError{Stderr: []byte("error: You must be logged in to the server\n")},
			"kubectl: error: You must be logged in to the server"},
		{"a missing binary is explained", exec.ErrNotFound, "kubectl not found in PATH"},
		{"other errors are wrapped", context.Canceled, "kubectl: context canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			client := NewClient("demo", "")
			fakeKubectl(st, client, nil, nil, tt.err)

			_, listErr := client.ListApps(st.Context())
			_, restartErr := client.RestartApp(st.Context(), "load-generator")

			require.Error(st, listErr)
			assert.Equal(st, tt.wantMsg, listErr.Error())
			require.Error(st, restartErr)
			assert.Equal(st, tt.wantMsg, restartErr.Error())
		})
	}
}
