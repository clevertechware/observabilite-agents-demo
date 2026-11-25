package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func run(t *testing.T, variables map[string]string, args ...string) (string, error) {
	t.Helper()

	root := newRootCommand(cmdtest.IsolatedEnvironment(t, variables))

	return cmdtest.RunRoot(t.Context(), root, args...)
}

func TestRoot_WhenOutputFormatIsUnknown_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := run(t, nil, "-o", "yaml", "config", "view")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
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
