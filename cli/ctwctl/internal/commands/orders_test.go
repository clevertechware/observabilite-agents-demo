package commands

import (
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	ordermocks "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order/mocks"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/shop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func ordersDeps(client order.Client) dependencies {
	return dependencies{newOrderClient: func(settings) order.Client { return client }}
}

var createdOrder = order.Order{
	ID: 42, SKU: "SKU-001", Quantity: 2, UnitPrice: "9.90", Total: "19.80", Location: "/orders/42",
}

func TestOrdersCreate_SendsSkuAndQuantityAndPrintsTheCreatedOrder(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)
	client.EXPECT().Create(mock.Anything, "SKU-001", 2).Return(createdOrder, nil)

	out, err := run(t, ordersDeps(client), "orders", "create", "--sku", "SKU-001", "--quantity", "2")

	require.NoError(t, err)
	assert.Equal(t, "order 42 created: SKU-001 x2, unit price 9.90, total 19.80\nLocation: /orders/42\n", out)
}

func TestOrdersCreate_WithJSONOutput_PrintsTheOrderWithItsLocation(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)
	client.EXPECT().Create(mock.Anything, "SKU-001", 2).Return(createdOrder, nil)

	out, err := run(t, ordersDeps(client), "orders", "create", "--sku", "SKU-001", "--quantity", "2", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t,
		`{"id":42,"sku":"SKU-001","quantity":2,"unitPrice":9.90,"total":19.80,"location":"/orders/42"}`, out)
}

func TestOrdersCreate_WhenStockIsInsufficient_ReturnsTheTypedError(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)
	client.EXPECT().Create(mock.Anything, "SKU-LOW", 5).Return(order.Order{}, shop.ErrInsufficientStock)

	_, err := run(t, ordersDeps(client), "orders", "create", "--sku", "SKU-LOW", "--quantity", "5")

	assert.ErrorIs(t, err, shop.ErrInsufficientStock)
}

func TestOrdersCreate_WithoutSku_FailsBeforeAnyCall(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)

	_, err := run(t, ordersDeps(client), "orders", "create")

	assert.Error(t, err)
}

func TestOrdersCreate_WithNonPositiveQuantity_FailsBeforeAnyCall(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)

	_, err := run(t, ordersDeps(client), "orders", "create", "--sku", "SKU-001", "--quantity", "0")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--quantity")
}

func TestOrdersGet_PrintsTheOrderReadByItsIdentifier(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)
	client.EXPECT().Get(mock.Anything, int64(42)).Return(order.Order{
		ID: 42, SKU: "SKU-001", Quantity: 2, UnitPrice: "9.90", Total: "19.80",
	}, nil)

	out, err := run(t, ordersDeps(client), "orders", "get", "42")

	require.NoError(t, err)
	assert.Equal(t, "order 42 read: SKU-001 x2, unit price 9.90, total 19.80\n", out)
}

func TestOrdersGet_WithJSONOutput_OmitsTheLocation(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)
	client.EXPECT().Get(mock.Anything, int64(42)).Return(order.Order{
		ID: 42, SKU: "SKU-001", Quantity: 2, UnitPrice: "9.90", Total: "19.80",
	}, nil)

	out, err := run(t, ordersDeps(client), "orders", "get", "42", "-o", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `{"id":42,"sku":"SKU-001","quantity":2,"unitPrice":9.90,"total":19.80}`, out)
}

func TestOrdersGet_WithNonNumericIdentifier_FailsBeforeAnyCall(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)

	_, err := run(t, ordersDeps(client), "orders", "get", "abc")

	assert.Error(t, err)
}

func TestOrdersGet_WhenOrderIsMissing_ReturnsNotFound(t *testing.T) {
	t.Parallel()

	client := ordermocks.NewClient(t)
	client.EXPECT().Get(mock.Anything, int64(7)).Return(order.Order{}, shop.ErrNotFound)

	_, err := run(t, ordersDeps(client), "orders", "get", "7")

	assert.ErrorIs(t, err, shop.ErrNotFound)
}

func TestOrders_UseTheShopURLFromTheConfiguration(t *testing.T) {
	t.Parallel()

	var gotURL string
	client := ordermocks.NewClient(t)
	client.EXPECT().Get(mock.Anything, int64(1)).Return(order.Order{ID: 1}, nil)
	deps := dependencies{
		newOrderClient: func(s settings) order.Client {
			gotURL = s.Shop.URL
			return client
		},
		environment: isolatedEnvironment(t, map[string]string{"CTWCTL_SHOP_URL": "https://shop.example.test"}),
	}

	_, err := run(t, deps, "orders", "get", "1")

	require.NoError(t, err)
	assert.Equal(t, "https://shop.example.test", gotURL)
}
