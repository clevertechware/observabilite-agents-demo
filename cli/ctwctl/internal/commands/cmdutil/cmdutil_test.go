package cmdutil_test

import (
	"context"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func loadSettings(t *testing.T, environment config.Environment, args ...string) (config.Settings, error) {
	t.Helper()

	load := cmdutil.NewSettingsLoader(environment)
	var settings config.Settings
	var loadErr error
	probe := &cli.Command{
		Name: "probe",
		Action: func(_ context.Context, cmd *cli.Command) error {
			settings, loadErr = load(cmd)
			return nil
		},
	}

	_, err := cmdtest.Run(t.Context(), probe, args...)
	require.NoError(t, err)

	return settings, loadErr
}

func TestSettingsLoader_AppliesTheEnvironmentWhenNoFlagOverridesIt(t *testing.T) {
	t.Parallel()

	environment := cmdtest.IsolatedEnvironment(t, map[string]string{"CTWCTL_SHOP_URL": "https://shop.example.test"})

	settings, err := loadSettings(t, environment, "probe")

	require.NoError(t, err)
	assert.Equal(t, "https://shop.example.test", settings.Shop.URL)
}

func TestSettingsLoader_GlobalFlagsOverrideTheEnvironmentWhereverTheyAreGiven(t *testing.T) {
	t.Parallel()

	environment := cmdtest.IsolatedEnvironment(t, map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"})

	beforeTheCommand, err := loadSettings(t, environment, "--namespace", "from-flag", "probe")
	require.NoError(t, err)
	afterTheCommand, err := loadSettings(t, environment, "probe", "--namespace", "from-flag", "-o", "json")
	require.NoError(t, err)

	assert.Equal(t, "from-flag", beforeTheCommand.Kube.Namespace)
	assert.Equal(t, "from-flag", afterTheCommand.Kube.Namespace)
	assert.Equal(t, cmdutil.OutputJSON, afterTheCommand.Output)
}

func TestSettingsLoader_WhenOutputFormatIsUnknown_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := loadSettings(t, cmdtest.IsolatedEnvironment(t, nil), "probe", "-o", "yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}
