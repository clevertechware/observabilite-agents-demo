package commands

import (
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	orderscmd "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/orders"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	ordermocks "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRoot_WhenOutputFormatIsUnknown_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := run(t, dependencies{}, "-o", "yaml", "config", "view")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

func TestRoot_LoadsTheOrderSettingsFromTheEnvironmentAndTheFlags(t *testing.T) {
	t.Parallel()

	var got config.Settings
	client := ordermocks.NewClient(t)
	client.EXPECT().Get(mock.Anything, int64(1)).Return(order.Order{ID: 1}, nil)
	deps := dependencies{
		environment: cmdtest.IsolatedEnvironment(t, map[string]string{"CTWCTL_SHOP_URL": "https://shop.example.test"}),
		orders: orderscmd.Dependencies{NewOrderClient: func(s config.Settings) order.Client {
			got = s
			return client
		}},
	}

	_, err := run(t, deps, "orders", "get", "1", "-o", "json")

	require.NoError(t, err)
	assert.Equal(t, "https://shop.example.test", got.Shop.URL)
	assert.Equal(t, "json", got.Output)
}
