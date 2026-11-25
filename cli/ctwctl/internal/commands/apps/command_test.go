package apps

import (
	"errors"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	appmocks "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app/mocks"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

var appStatuses = []app.Status{
	{Name: "order-service", Ready: 1, Replicas: 1, Available: true},
	{Name: "pricing-service", Ready: 1, Replicas: 2, Available: false},
}

var (
	textOutput = config.Settings{Output: config.OutputText}
	jsonOutput = config.Settings{Output: config.OutputJSON}
)

func appDeps(client app.Client, settings config.Settings) Dependencies {
	return Dependencies{
		LoadSettings: func(*cli.Command) (config.Settings, error) { return settings, nil },
		NewAppClient: func(config.Settings) app.Client { return client },
	}
}

func TestAppsList_WithJSONOutput_PrintsOneEntryPerApp(t *testing.T) {
	t.Parallel()

	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(appStatuses, nil)

	out, err := run(t, appDeps(client, jsonOutput), "apps", "list")

	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"name":"order-service","ready":1,"replicas":1,"available":true},
		{"name":"pricing-service","ready":1,"replicas":2,"available":false}]`, out)
}

func TestAppsList_WithTextOutput_PrintsReadyOverDesiredAndAvailability(t *testing.T) {
	t.Parallel()

	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(appStatuses, nil)

	out, err := run(t, appDeps(client, textOutput), "apps", "list")

	require.NoError(t, err)
	assert.Equal(t, "NAME             READY  AVAILABLE\n"+
		"order-service    1/1    yes\n"+
		"pricing-service  1/2    no\n", out)
}

func TestAppsList_BuildsTheClientFromTheLoadedSettings(t *testing.T) {
	t.Parallel()

	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(nil, nil)
	loaded := config.Settings{
		Output: config.OutputText,
		Kube:   config.KubeSettings{Namespace: "staging", Context: "prod-cluster"},
	}
	var got config.Settings
	deps := Dependencies{
		LoadSettings: func(*cli.Command) (config.Settings, error) { return loaded, nil },
		NewAppClient: func(s config.Settings) app.Client {
			got = s
			return client
		},
	}

	_, err := run(t, deps, "apps", "list")

	require.NoError(t, err)
	assert.Equal(t, loaded, got)
}

func TestApps_WhenSettingsCannotBeLoaded_ReturnTheErrorWithoutCallingTheClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"list", []string{"list"}},
		{"restart", []string{"restart", "load-generator"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			loadErr := errors.New("read config.toml: invalid TOML")
			deps := Dependencies{
				LoadSettings: func(*cli.Command) (config.Settings, error) { return config.Settings{}, loadErr },
				NewAppClient: func(config.Settings) app.Client {
					st.Fatal("the client should not be built without settings")
					return nil
				},
			}

			_, err := run(st, deps, append([]string{"apps"}, tt.args...)...)

			assert.ErrorIs(st, err, loadErr)
		})
	}
}

func TestAppsList_WhenClientFails_ReturnsItsError(t *testing.T) {
	t.Parallel()

	client := appmocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(nil, errors.New("kubectl : You must be logged in to the server"))

	_, err := run(t, appDeps(client, textOutput), "apps", "list")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "You must be logged in to the server")
}

func TestAppsRestart_RestartsTheNamedAppAndPrintsTheOutput(t *testing.T) {
	t.Parallel()

	client := appmocks.NewClient(t)
	client.EXPECT().RestartApp(mock.Anything, "load-generator").
		Return("deployment.apps/load-generator restarted\n", nil)

	out, err := run(t, appDeps(client, textOutput), "apps", "restart", "load-generator")

	require.NoError(t, err)
	assert.Equal(t, "deployment.apps/load-generator restarted\n", out)
}

func TestAppsRestart_WithJSONOutput_ReportsTheRestartedApp(t *testing.T) {
	t.Parallel()

	client := appmocks.NewClient(t)
	client.EXPECT().RestartApp(mock.Anything, "load-generator").Return("restarted", nil)

	out, err := run(t, appDeps(client, jsonOutput), "apps", "restart", "load-generator")

	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"load-generator","restarted":true}`, out)
}

func TestAppsRestart_WhenClientFails_ReturnsItsError(t *testing.T) {
	t.Parallel()

	client := appmocks.NewClient(t)
	client.EXPECT().RestartApp(mock.Anything, "load-generator").Return("", errors.New("kubectl not found in PATH"))

	_, err := run(t, appDeps(client, textOutput), "apps", "restart", "load-generator")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in PATH")
}

func TestAppsRestart_RejectsInvalidNamesWithoutCallingTheClient(t *testing.T) {
	t.Parallel()

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
			st.Parallel()

			client := appmocks.NewClient(st)

			_, err := run(st, appDeps(client, textOutput), append([]string{"apps", "restart", "--"}, tt.args...)...)

			assert.Error(st, err)
		})
	}
}
