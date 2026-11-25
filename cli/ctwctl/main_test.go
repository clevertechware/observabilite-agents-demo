package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func isolatedEnvironment(t *testing.T, variables map[string]string) config.Environment {
	t.Helper()

	all := map[string]string{"XDG_CONFIG_HOME": t.TempDir()}
	maps.Copy(all, variables)

	return config.Environment{Variables: all, UserHomeDir: func() (string, error) {
		t.Fatal("user home should not be requested when XDG_CONFIG_HOME is set")
		return "", nil
	}}
}

func writeConfigFile(t *testing.T, environment config.Environment, content string) {
	t.Helper()

	path := filepath.Join(environment.Variables["XDG_CONFIG_HOME"], "ctwctl", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func runIn(t *testing.T, environment config.Environment, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := newRootCommand(environment)
	root.Writer = &out
	root.ErrWriter = io.Discard
	root.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := root.Run(t.Context(), append([]string{root.Name}, args...))

	return out.String(), err
}

func run(t *testing.T, variables map[string]string, args ...string) (string, error) {
	t.Helper()

	return runIn(t, isolatedEnvironment(t, variables), args...)
}

func TestRoot_WhenOutputFormatIsUnknown_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := run(t, nil, "-o", "yaml", "config", "view")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

func TestRoot_ConfigurationSourcesOverrideEachOtherInTheDocumentedOrder(t *testing.T) {
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
			"a global flag before the command overrides the environment and the file",
			"[kube]\nnamespace = \"from-file\"\n",
			map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"},
			[]string{"--namespace", "from-flag", "config", "view"}, "from-flag",
		},
		{
			"a global flag after the command overrides the environment",
			"",
			map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"},
			[]string{"config", "view", "--namespace", "from-flag"}, "from-flag",
		},
		{
			"a flag that is not set does not erase the environment value",
			"",
			map[string]string{"CTWCTL_KUBE_NAMESPACE": "from-env"},
			[]string{"--context", "other", "config", "view"}, "from-env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			environment := isolatedEnvironment(st, tt.env)
			if tt.file != "" {
				writeConfigFile(st, environment, tt.file)
			}
			args := tt.args
			if args == nil {
				args = []string{"config", "view"}
			}

			out, err := runIn(st, environment, append(args, "-o", "json")...)

			require.NoError(st, err)
			var view config.Settings
			require.NoError(st, json.Unmarshal([]byte(out), &view))
			assert.Equal(st, tt.wantNamespace, view.Kube.Namespace)
		})
	}
}

func TestRoot_ConfigViewShowsOnlyTheKnownKeys(t *testing.T) {
	t.Parallel()

	environment := isolatedEnvironment(t, map[string]string{"CTWCTL_FOO_BAR": "from-env"})
	writeConfigFile(t, environment, "[o2]\nstream = \"from-file\"\n")

	out, err := runIn(t, environment, "config", "view", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"output": "json",
		"o2": {"url": "https://o2.example.com", "org": "default", "user": "", "token": ""},
		"shop": {"url": "https://shop.example.com"},
		"kube": {"namespace": "demo", "context": ""}
	}`, out)
}

func TestRoot_OrdersCallTheShopConfiguredInTheEnvironment(t *testing.T) {
	t.Parallel()

	shopServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/orders/42", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":42,"sku":"SKU-001","quantity":2,"unitPrice":9.90,"total":19.80}`))
	}))
	defer shopServer.Close()

	out, err := run(t, map[string]string{"CTWCTL_SHOP_URL": shopServer.URL}, "orders", "get", "42", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `{"id":42,"sku":"SKU-001","quantity":2,"unitPrice":9.90,"total":19.80}`, out)
}

func TestRoot_O2CallsTheOpenObserveConfiguredInTheEnvironmentWithItsCredentials(t *testing.T) {
	t.Parallel()

	o2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, token, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "ctwctl@sa.internal", user)
		assert.Equal(t, "jeton-de-test", token)
		assert.Equal(t, "/api/demo-org/streams", r.URL.Path)
		_, _ = w.Write([]byte(`{"list":[{"name":"default","stream_type":"logs","schema":[{"name":"body"}]}]}`))
	}))
	defer o2Server.Close()

	out, err := run(t, map[string]string{
		"CTWCTL_O2_URL":   o2Server.URL,
		"CTWCTL_O2_ORG":   "demo-org",
		"CTWCTL_O2_USER":  "ctwctl@sa.internal",
		"CTWCTL_O2_TOKEN": "jeton-de-test",
	}, "o2", "streams", "--type", "logs")

	require.NoError(t, err)
	assert.Contains(t, out, "default")
}
