package config

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isolatedEnvironment(t *testing.T, variables map[string]string) Environment {
	t.Helper()
	all := map[string]string{"XDG_CONFIG_HOME": t.TempDir()}
	maps.Copy(all, variables)

	return Environment{Variables: all, UserHomeDir: func() (string, error) {
		t.Fatal("user home should not be requested when XDG_CONFIG_HOME is set")
		return "", nil
	}}
}

func TestLoad_AppliesSourcesInPriorityOrder(t *testing.T) {
	t.Parallel()
	environment := isolatedEnvironment(t, map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"})
	dir := environment.Variables["XDG_CONFIG_HOME"]
	path := filepath.Join(dir, "ctwctl", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("[kube]\nnamespace = \"from-file\"\n"), 0o600))

	resolved, err := Load(environment, map[string]any{"kube.namespace": "from-flag"})

	require.NoError(t, err)
	assert.Equal(t, "from-flag", resolved.Settings.Kube.Namespace)
}

func TestLoad_UsesEnvironmentWhenNoFlagOverridesIt(t *testing.T) {
	t.Parallel()
	environment := isolatedEnvironment(t, map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"})

	resolved, err := Load(environment, nil)

	require.NoError(t, err)
	assert.Equal(t, "from-env", resolved.Settings.Kube.Namespace)
}

func TestResolved_MaskedViewHidesTokenButKeepsEffectiveSettings(t *testing.T) {
	t.Parallel()
	environment := isolatedEnvironment(t, map[string]string{"CTWCTL_O2_TOKEN": "secret"})

	resolved, err := Load(environment, nil)
	require.NoError(t, err)
	view, err := resolved.MaskedView()

	require.NoError(t, err)
	assert.Equal(t, "secret", resolved.Settings.O2.Token)
	assert.Equal(t, "********", view.Values["o2.token"])
	assert.Equal(t, "********", view.Raw["o2"].(map[string]any)["token"])
}

func TestPath_UsesXDGConfigHomeWhenSet(t *testing.T) {
	t.Parallel()
	environment := isolatedEnvironment(t, map[string]string{"XDG_CONFIG_HOME": "/tmp/xdg"})

	got, err := configPath(environment)

	require.NoError(t, err)
	assert.Equal(t, "/tmp/xdg/ctwctl/config.toml", got)
}

func TestPath_FallsBackToDotConfigUnderUserHome(t *testing.T) {
	t.Parallel()
	environment := Environment{UserHomeDir: func() (string, error) { return "/home/dev", nil }}

	got, err := configPath(environment)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/home/dev", ".config", "ctwctl", "config.toml"), got)
}

func TestEnvKey_StripsPrefixLowercasesAndSplitsOnUnderscore(t *testing.T) {
	t.Parallel()
	key, value := envKey("CTWCTL_O2_TOKEN", "secret")

	assert.Equal(t, "o2.token", key)
	assert.Equal(t, "secret", value)
}

func TestProcessEnvironment_ReadsTheVariablesOfTheRunningProcess(t *testing.T) {
	t.Setenv("CTWCTL_O2_URL", "https://o2.example.test?a=b")

	environment := ProcessEnvironment()

	assert.Equal(t, "https://o2.example.test?a=b", environment.Variables["CTWCTL_O2_URL"])
}
