package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfigFile(t *testing.T, environment config.Environment, content string) {
	t.Helper()
	path := configFilePath(environment)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func resolvedConfig(t *testing.T, environment config.Environment, args ...string) map[string]any {
	t.Helper()
	out, err := run(t, dependencies{environment: environment}, append(args, "config", "view", "-o", "json")...)
	require.NoError(t, err)
	var view map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &view))
	return view
}

func namespaceOf(view map[string]any) any {
	return view["kube"].(map[string]any)["namespace"]
}

func TestLoadConfig_SourcesOverrideEachOtherInTheDocumentedOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		file          string
		env           map[string]string
		args          []string
		wantNamespace string
	}{
		{"defaults alone give the demo namespace", "", nil, nil, "demo"},
		{"the file overrides the defaults", "[kube]\nnamespace = \"from-file\"\n", nil, nil, "from-file"},
		{
			"the environment overrides the file",
			"[kube]\nnamespace = \"from-file\"\n",
			map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"}, nil, "from-env",
		},
		{
			"a flag overrides the environment and the file",
			"[kube]\nnamespace = \"from-file\"\n",
			map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"},
			[]string{"--namespace", "from-flag"}, "from-flag",
		},
		{
			"a flag that is not set does not erase the environment value",
			"",
			map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"},
			[]string{"--context", "other"}, "from-env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			environment := isolatedEnvironment(st, tt.env)
			if tt.file != "" {
				writeConfigFile(st, environment, tt.file)
			}

			view := resolvedConfig(st, environment, tt.args...)

			assert.Equal(st, tt.wantNamespace, namespaceOf(view))
		})
	}
}

func TestLoadConfig_EnvironmentVariableMapsToTheDottedKey(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, map[string]string{"CTWCTL_O2_URL": "https://o2.example.test"})

	view := resolvedConfig(t, environment)

	assert.Equal(t, "https://o2.example.test", view["o2"].(map[string]any)["url"])
}

func TestLoadConfig_WhenFileIsNotValidTOML_NamesTheFile(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)
	writeConfigFile(t, environment, "ceci n'est pas du toml [")

	_, err := run(t, dependencies{environment: environment}, "config", "view")

	require.Error(t, err)
	assert.Contains(t, err.Error(), configFilePath(environment))
}

func TestRoot_WhenOutputFormatIsUnknown_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := run(t, dependencies{}, "-o", "yaml", "config", "view")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

func TestConfigSet_WritesTheFileWithOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)

	out, err := run(t, dependencies{environment: environment}, "config", "set", "o2.token", "jeton-secret")

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

	_, err := run(t, dependencies{environment: environment}, "config", "set", "kube.namespace", "staging")

	require.NoError(t, err)
	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestConfigSet_KeepsValuesAlreadyInTheFile(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)
	writeConfigFile(t, environment, "[o2]\nurl = \"https://o2.example.test\"\n")

	_, err := run(t, dependencies{environment: environment}, "config", "set", "kube.namespace", "staging")

	require.NoError(t, err)
	view := resolvedConfig(t, environment)
	assert.Equal(t, "https://o2.example.test", view["o2"].(map[string]any)["url"])
	assert.Equal(t, "staging", namespaceOf(view))
}

func TestConfigSet_DoesNotPersistEnvironmentValuesInTheFile(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, map[string]string{"CTWCTL_O2_TOKEN": "jeton-de-l-environnement"})

	_, err := run(t, dependencies{environment: environment}, "config", "set", "kube.namespace", "staging")

	require.NoError(t, err)
	content, readErr := os.ReadFile(configFilePath(environment))
	require.NoError(t, readErr)
	assert.NotContains(t, string(content), "jeton-de-l-environnement")
}

func TestConfigSet_RejectsUnknownKeysAndListsTheKnownOnes(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, nil)

	_, err := run(t, dependencies{environment: environment}, "config", "set", "o2.stream.type", "logs")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "o2.token")
	assert.NoFileExists(t, configFilePath(environment))
}

func TestConfigSet_WithoutValue_ReturnsUsageError(t *testing.T) {
	t.Parallel()

	_, err := run(t, dependencies{}, "config", "set", "o2.token")

	require.Error(t, err)
}

func TestConfigView_MasksTheTokenInTextAndJSON(t *testing.T) {
	t.Parallel()

	deps := dependencies{environment: isolatedEnvironment(t, map[string]string{"CTWCTL_O2_TOKEN": "jeton-secret"})}

	text, err := run(t, deps, "config", "view")
	require.NoError(t, err)
	jsonOut, err := run(t, deps, "config", "view", "-o", "json")
	require.NoError(t, err)

	assert.NotContains(t, text, "jeton-secret")
	assert.NotContains(t, jsonOut, "jeton-secret")
	assert.Contains(t, text, "o2.token = ********")
}

func TestConfigView_WithTextOutput_PrintsOneSortedKeyPerLine(t *testing.T) {
	t.Parallel()

	out, err := run(t, dependencies{}, "config", "view")

	require.NoError(t, err)
	assert.Equal(t, "kube.context = \n"+
		"kube.namespace = demo\n"+
		"o2.org = default\n"+
		"o2.token = \n"+
		"o2.url = https://o2.example.com\n"+
		"o2.user = \n"+
		"output = text\n"+
		"shop.url = https://shop.example.com\n", out)
}
