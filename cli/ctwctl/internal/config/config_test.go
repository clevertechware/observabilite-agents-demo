package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_AppliesSourcesInPriorityOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("CTWCTL_KUBE_NAMESPACE", "from-env")
	path := filepath.Join(dir, "ctwctl", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("[kube]\nnamespace = \"from-file\"\n"), 0o600))

	resolved, err := Load(map[string]any{"kube.namespace": "from-flag"})

	require.NoError(t, err)
	assert.Equal(t, "from-flag", resolved.Settings.Kube.Namespace)
}

func TestLoad_UsesEnvironmentWhenNoFlagOverridesIt(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CTWCTL_KUBE_NAMESPACE", "from-env")

	resolved, err := Load(nil)

	require.NoError(t, err)
	assert.Equal(t, "from-env", resolved.Settings.Kube.Namespace)
}

func TestResolved_MaskedViewHidesTokenButKeepsEffectiveSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CTWCTL_O2_TOKEN", "secret")

	resolved, err := Load(nil)
	require.NoError(t, err)
	view, err := resolved.MaskedView()

	require.NoError(t, err)
	assert.Equal(t, "secret", resolved.Settings.O2.Token)
	assert.Equal(t, "********", view.Values["o2.token"])
	assert.Equal(t, "********", view.Raw["o2"].(map[string]any)["token"])
}

func TestPath_UsesXDGConfigHomeWhenSet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")

	got, err := configPath(func() (string, error) {
		t.Fatal("user home should not be requested when XDG_CONFIG_HOME is set")
		return "", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "/tmp/xdg/ctwctl/config.toml", got)
}

func TestPath_FallsBackToDotConfigUnderUserHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	got, err := configPath(func() (string, error) { return "/home/dev", nil })

	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/home/dev", ".config", "ctwctl", "config.toml"), got)
}

func TestEnvKey_StripsPrefixLowercasesAndSplitsOnUnderscore(t *testing.T) {
	key, value := envKey("CTWCTL_O2_TOKEN", "secret")

	assert.Equal(t, "o2.token", key)
	assert.Equal(t, "secret", value)
}
