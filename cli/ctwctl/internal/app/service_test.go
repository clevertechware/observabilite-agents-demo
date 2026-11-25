package app_test

import (
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAppsRestart_RejectsInvalidNamesWithoutCallingClient(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "--all", "statefulset/postgres", "Order"} {
		t.Run(name, func(st *testing.T) {
			st.Parallel()

			client := mocks.NewClient(st)

			_, err := app.NewApps(client).Restart(st.Context(), name)

			assert.ErrorIs(st, err, app.ErrInvalidName)
		})
	}
}

func TestAppsRestart_DelegatesValidName(t *testing.T) {
	t.Parallel()

	client := mocks.NewClient(t)
	client.EXPECT().RestartApp(mock.Anything, "load-generator").Return("restarted", nil)

	result, err := app.NewApps(client).Restart(t.Context(), "load-generator")

	require.NoError(t, err)
	assert.Equal(t, "restarted", result)
}

func TestAppsList_DelegatesToClient(t *testing.T) {
	t.Parallel()

	want := []app.Status{{Name: "order-service", Ready: 1, Replicas: 1, Available: true}}
	client := mocks.NewClient(t)
	client.EXPECT().ListApps(mock.Anything).Return(want, nil)

	got, err := app.NewApps(client).List(t.Context())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}
