package order_test

import (
	"strconv"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrdersCreate_RejectsNonPositiveQuantitiesWithoutCallingClient(t *testing.T) {
	t.Parallel()
	for _, quantity := range []int{0, -1} {
		t.Run(strconv.Itoa(quantity), func(st *testing.T) {
			st.Parallel()
			client := mocks.NewClient(st)

			_, err := order.NewOrders(client).Create(st.Context(), "SKU-001", quantity)

			assert.ErrorIs(st, err, order.ErrInvalidQuantity)
		})
	}
}

func TestOrdersCreate_DelegatesValidOrder(t *testing.T) {
	t.Parallel()
	want := order.Order{ID: 42, SKU: "SKU-001", Quantity: 2}
	client := mocks.NewClient(t)
	client.EXPECT().Create(mock.Anything, "SKU-001", 2).Return(want, nil)

	got, err := order.NewOrders(client).Create(t.Context(), "SKU-001", 2)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestOrdersGet_DelegatesToClient(t *testing.T) {
	t.Parallel()
	want := order.Order{ID: 42, SKU: "SKU-001", Quantity: 2}
	client := mocks.NewClient(t)
	client.EXPECT().Get(mock.Anything, int64(42)).Return(want, nil)

	got, err := order.NewOrders(client).Get(t.Context(), 42)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}
