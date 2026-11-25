package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func writeConfigFile(t *testing.T, environment config.Environment, content string) {
	t.Helper()
	path := configFilePath(environment)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

var viewedSettings = config.Settings{
	Output: config.OutputText,
	O2:     config.O2Settings{URL: "https://o2.example.test", Org: "default", User: "ctwctl", Token: "jeton-secret"},
	Shop:   config.ShopSettings{URL: "https://shop.example.test"},
	Kube:   config.KubeSettings{Namespace: "demo"},
}

func TestConfigSet_WritesTheFileWithOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)

	out, err := runSet(t, environment, "o2.token", "jeton-secret")

	require.NoError(t, err)
	path := configFilePath(environment)
	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.Contains(t, out, path)
	assert.NotContains(t, out, "jeton-secret")
}

func TestConfigSet_TightensPermissionsOfAnExistingOpenFile(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)
	writeConfigFile(t, environment, "[o2]\nurl = \"https://o2.example.test\"\n")
	path := configFilePath(environment)
	require.NoError(t, os.Chmod(path, 0o644))

	_, err := runSet(t, environment, "kube.namespace", "staging")

	require.NoError(t, err)
	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestConfigSet_KeepsValuesAlreadyInTheFile(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)
	writeConfigFile(t, environment, "[o2]\nurl = \"https://o2.example.test\"\n")

	_, err := runSet(t, environment, "kube.namespace", "staging")

	require.NoError(t, err)
	settings, loadErr := config.Load(environment, nil)
	require.NoError(t, loadErr)
	assert.Equal(t, "https://o2.example.test", settings.O2.URL)
	assert.Equal(t, "staging", settings.Kube.Namespace)
}

func TestConfigSet_DoesNotPersistEnvironmentValuesInTheFile(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, map[string]string{"CTWCTL_O2_TOKEN": "jeton-de-l-environnement"})

	_, err := runSet(t, environment, "kube.namespace", "staging")

	require.NoError(t, err)
	content, readErr := os.ReadFile(configFilePath(environment))
	require.NoError(t, readErr)
	assert.NotContains(t, string(content), "jeton-de-l-environnement")
}

func TestConfigSet_RejectsUnknownKeysAndListsTheKnownOnes(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)

	_, err := runSet(t, environment, "o2.stream.type", "logs")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "o2.token")
	assert.NoFileExists(t, configFilePath(environment))
}

func TestConfigSet_WithoutValue_ReturnsUsageError(t *testing.T) {
	t.Parallel()

	_, err := runSet(t, isolatedEnvironment(t, nil), "o2.token")

	require.Error(t, err)
}

func TestConfigView_MasksTheTokenInTextAndJSON(t *testing.T) {
	t.Parallel()

	asJSON := viewedSettings
	asJSON.Output = config.OutputJSON

	text, err := runView(t, viewedSettings)
	require.NoError(t, err)
	jsonOut, err := runView(t, asJSON)
	require.NoError(t, err)

	assert.NotContains(t, text, "jeton-secret")
	assert.NotContains(t, jsonOut, "jeton-secret")
	assert.Contains(t, text, "o2.token = ********")
}

func TestConfigView_WithTextOutput_PrintsOneSortedKeyPerLine(t *testing.T) {
	t.Parallel()

	out, err := runView(t, viewedSettings)

	require.NoError(t, err)
	assert.Equal(t, "kube.context = \n"+
		"kube.namespace = demo\n"+
		"o2.org = default\n"+
		"o2.token = ********\n"+
		"o2.url = https://o2.example.test\n"+
		"o2.user = ctwctl\n"+
		"output = text\n"+
		"shop.url = https://shop.example.test\n", out)
}

func TestConfigView_WithJSONOutput_PrintsTheMaskedSettings(t *testing.T) {
	t.Parallel()

	asJSON := viewedSettings
	asJSON.Output = config.OutputJSON

	out, err := runView(t, asJSON)

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"output": "json",
		"o2": {"url": "https://o2.example.test", "org": "default", "user": "ctwctl", "token": "********"},
		"shop": {"url": "https://shop.example.test"},
		"kube": {"namespace": "demo", "context": ""}
	}`, out)
}

func TestConfigView_WhenSettingsCannotBeLoaded_ReturnsTheError(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("read config.toml: invalid TOML")
	command := NewCommand(config.Environment{}, func(*cli.Command) (config.Settings, error) {
		return config.Settings{}, loadErr
	})

	_, err := run(t, command, "config", "view")

	assert.ErrorIs(t, err, loadErr)
}
