package commands

import (
	"errors"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	appmocks "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var appStatuses = []app.Status{
	{Name: "order-service", Ready: 1, Replicas: 1, Available: true},
	{Name: "pricing-service", Ready: 1, Replicas: 2, Available: false},
}

func appDeps(client app.Client) dependencies {
	return dependencies{newAppClient: func(settings) app.Client { return client }}
}

func TestAppsList_WithJSONOutput_PrintsOneEntryPerApp(t *testing.T) {
	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(appStatuses, nil)

	out, err := run(t, appDeps(client), "apps", "list", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"name":"order-service","ready":1,"replicas":1,"available":true},
		{"name":"pricing-service","ready":1,"replicas":2,"available":false}]`, out)
}

func TestAppsList_WithTextOutput_PrintsReadyOverDesiredAndAvailability(t *testing.T) {
	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(appStatuses, nil)

	out, err := run(t, appDeps(client), "apps", "list")

	require.NoError(t, err)
	assert.Equal(t, "NOM              PRÊTS  DISPONIBLE\n"+
		"order-service    1/1    oui\n"+
		"pricing-service  1/2    non\n", out)
}

func TestAppsList_BuildsTheClientFromNamespaceAndContextFlags(t *testing.T) {
	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(nil, nil)
	var got settings
	deps := dependencies{newAppClient: func(s settings) app.Client {
		got = s
		return client
	}}

	_, err := run(t, deps, "--namespace", "staging", "--context", "prod-cluster", "apps", "list")

	require.NoError(t, err)
	assert.Equal(t, "staging", got.Kube.Namespace)
	assert.Equal(t, "prod-cluster", got.Kube.Context)
}

func TestAppsList_WhenClientFails_ReturnsItsError(t *testing.T) {
	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(nil, errors.New("kubectl : You must be logged in to the server"))

	_, err := run(t, appDeps(client), "apps", "list")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "You must be logged in to the server")
}

func TestAppsRestart_RestartsTheNamedAppAndPrintsTheOutput(t *testing.T) {
	client := appmocks.NewClient(t)
	client.EXPECT().RestartApp(mock.Anything, "load-generator").
		Return("deployment.apps/load-generator restarted\n", nil)

	out, err := run(t, appDeps(client), "apps", "restart", "load-generator")

	require.NoError(t, err)
	assert.Equal(t, "deployment.apps/load-generator restarted\n", out)
}

func TestAppsRestart_WithJSONOutput_ReportsTheRestartedApp(t *testing.T) {
	client := appmocks.NewClient(t)
	client.EXPECT().RestartApp(mock.Anything, "load-generator").Return("restarted", nil)

	out, err := run(t, appDeps(client), "apps", "restart", "load-generator", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"load-generator","restarted":true}`, out)
}

func TestAppsRestart_WhenClientFails_ReturnsItsError(t *testing.T) {
	client := appmocks.NewClient(t)
	client.EXPECT().RestartApp(mock.Anything, "load-generator").Return("", errors.New("kubectl est introuvable dans le PATH"))

	_, err := run(t, appDeps(client), "apps", "restart", "load-generator")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "introuvable dans le PATH")
}

func TestAppsRestart_RejectsInvalidNamesWithoutCallingTheClient(t *testing.T) {
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
		t.Run(tt.name, func(st *testing.T) {
			client := appmocks.NewClient(st)

			_, err := run(st, appDeps(client), append([]string{"apps", "restart", "--"}, tt.args...)...)

			assert.Error(st, err)
		})
	}
}
