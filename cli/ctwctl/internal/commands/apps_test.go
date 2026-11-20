package commands

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

var deploymentsJSON = []byte(`{
  "kind": "List",
  "items": [
    {"metadata": {"name": "order-service"}, "spec": {"replicas": 1},
     "status": {"readyReplicas": 1, "conditions": [{"type": "Progressing", "status": "True"}, {"type": "Available", "status": "True"}]}},
    {"metadata": {"name": "inventory-service"}, "spec": {"replicas": 1},
     "status": {"readyReplicas": 1, "conditions": [{"type": "Available", "status": "True"}]}}
  ]
}`)

const expectedAppsJSON = `[
  {"name": "order-service", "ready": 1, "replicas": 1, "available": true},
  {"name": "inventory-service", "ready": 1, "replicas": 1, "available": true}
]`

func TestAppsList_WithJSONOutput_PrintsOneEntryPerDeployment(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().
		Run(mock.Anything, []string{"get", "deployments", "-n", "demo", "-o", "json"}).
		Return(deploymentsJSON, nil)
	var out bytes.Buffer
	cmd := newRootCommand(dependencies{kubectl: runner})
	cmd.Writer = &out
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := cmd.Run(t.Context(), []string{"ctwctl", "apps", "list", "-o", "json"})

	require.NoError(t, err)
	assert.JSONEq(t, expectedAppsJSON, out.String())
}

func TestAppsList_WithTextOutput_PrintsReadyOverDesiredAndAvailability(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().Run(mock.Anything, mock.Anything).Return(deploymentsJSON, nil)

	out, err := run(t, dependencies{kubectl: runner}, "apps", "list")

	require.NoError(t, err)
	assert.Equal(t, "NOM                PRÊTS  DISPONIBLE\n"+
		"order-service      1/1    oui\n"+
		"inventory-service  1/1    oui\n", out)
}

func TestAppsList_WhenRolloutIsIncomplete_ReportsPartialReadinessAndUnavailable(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().Run(mock.Anything, mock.Anything).Return([]byte(`{"items":[
		{"metadata":{"name":"pricing-service"},"spec":{"replicas":2},
		 "status":{"readyReplicas":1,"conditions":[{"type":"Available","status":"False"}]}},
		{"metadata":{"name":"load-generator"},"spec":{},"status":{}}]}`), nil)

	out, err := run(t, dependencies{kubectl: runner}, "apps", "list", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"name":"pricing-service","ready":1,"replicas":2,"available":false},
		{"name":"load-generator","ready":0,"replicas":1,"available":false}]`, out)
}

func TestAppsList_WhenNamespaceAndContextAreGiven_ForwardsThemToKubectl(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().
		Run(mock.Anything, []string{"get", "deployments", "-n", "staging", "-o", "json", "--context", "prod-cluster"}).
		Return([]byte(`{"items":[]}`), nil)

	out, err := run(t, dependencies{kubectl: runner},
		"--namespace", "staging", "--context", "prod-cluster", "apps", "list", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `[]`, out)
}

func TestAppsList_WhenKubectlFails_ReportsItsStderr(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().Run(mock.Anything, mock.Anything).
		Return(nil, &exec.ExitError{Stderr: []byte("error: You must be logged in to the server\n")})

	_, err := run(t, dependencies{kubectl: runner}, "apps", "list")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "You must be logged in to the server")
}

func TestAppsList_WhenKubectlIsNotInstalled_SaysSo(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().Run(mock.Anything, mock.Anything).Return(nil, exec.ErrNotFound)

	_, err := run(t, dependencies{kubectl: runner}, "apps", "list")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "introuvable dans le PATH")
}

func TestAppsList_WhenKubectlPrintsInvalidJSON_ReturnsDecodeError(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().Run(mock.Anything, mock.Anything).Return([]byte("pas du json"), nil)

	_, err := run(t, dependencies{kubectl: runner}, "apps", "list")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "décodage")
}

func TestAppsRestart_RunsRolloutRestartOnTheNamedDeployment(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().
		Run(mock.Anything, []string{"rollout", "restart", "deployment/load-generator", "-n", "demo"}).
		Return([]byte("deployment.apps/load-generator restarted\n"), nil)

	out, err := run(t, dependencies{kubectl: runner}, "apps", "restart", "load-generator")

	require.NoError(t, err)
	assert.Equal(t, "deployment.apps/load-generator restarted\n", out)
}

func TestAppsRestart_WithJSONOutput_ReportsTheRestartedDeployment(t *testing.T) {
	runner := mocks.NewKubectlRunner(t)
	runner.EXPECT().Run(mock.Anything, mock.Anything).Return([]byte("restarted"), nil)

	out, err := run(t, dependencies{kubectl: runner}, "apps", "restart", "load-generator", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"load-generator","restarted":true}`, out)
}

func TestAppsRestart_RejectsInvalidNamesWithoutCallingKubectl(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"a name starting with a dash would be read as a kubectl flag", []string{"--all"}},
		{"a name with a slash would target another resource kind", []string{"statefulset/postgres"}},
		{"an uppercase name is not a valid Deployment name", []string{"Order"}},
		{"a missing name is a usage error", nil},
		{"two names are a usage error", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := mocks.NewKubectlRunner(t)

			_, err := run(t, dependencies{kubectl: runner}, append([]string{"apps", "restart", "--"}, tt.args...)...)

			assert.Error(t, err)
		})
	}
}
