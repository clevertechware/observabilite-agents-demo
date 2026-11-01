package inventory_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type priceProviderFunc func(context.Context, string, int) (inventory.Price, error)

func (f priceProviderFunc) Price(ctx context.Context, sku string, quantity int) (inventory.Price, error) {
	return f(ctx, sku, quantity)
}

func TestReserveStockRejectsInvalidQuantity(t *testing.T) {
	t.Parallel()

	for _, quantity := range []int{0, -3} {
		t.Run(fmt.Sprint(quantity), func(t *testing.T) {
			t.Parallel()

			repository := mocks.NewMockStockRepository(t)
			provider := priceProviderFunc(func(context.Context, string, int) (inventory.Price, error) {
				t.Fatal("pricing must not be called for an invalid quantity")
				return inventory.Price{}, nil
			})
			reservations := inventory.NewReservations(repository, provider)

			reservation, err := reservations.ReserveStock(context.Background(), "SKU-42", quantity)

			require.ErrorIs(t, err, inventory.ErrInvalidQuantity)
			assert.Empty(t, reservation)
			repository.AssertNotCalled(t, "Quantity")
			repository.AssertNotCalled(t, "Reserve")
		})
	}
}

func TestReserveStock(t *testing.T) {
	t.Parallel()

	dependencyErr := errors.New("dependency unavailable")
	tests := []struct {
		name        string
		available   int
		quantityErr error
		priceErr    error
		reserveErr  error
		wantPricing bool
		wantReserve bool
		wantErr     error
	}{
		{name: "success", available: 10, wantPricing: true, wantReserve: true},
		{name: "stock lookup fails", quantityErr: dependencyErr, wantErr: dependencyErr},
		{name: "insufficient stock before pricing", available: 1, wantErr: inventory.ErrInsufficientStock},
		{name: "pricing fails without decrement", available: 10, priceErr: dependencyErr, wantPricing: true, wantErr: dependencyErr},
		{name: "unknown price maps to unknown sku", available: 10, priceErr: fmt.Errorf("pricing: %w", inventory.ErrPriceNotFound), wantPricing: true, wantErr: inventory.ErrSkuNotFound},
		{name: "stock becomes insufficient after pricing", available: 10, reserveErr: fmt.Errorf("reserve: %w", inventory.ErrInsufficientStock), wantPricing: true, wantReserve: true, wantErr: inventory.ErrInsufficientStock},
		{name: "reservation fails", available: 10, reserveErr: dependencyErr, wantPricing: true, wantReserve: true, wantErr: dependencyErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			repository := mocks.NewMockStockRepository(t)
			repository.EXPECT().Quantity(ctx, "SKU-42").Return(tt.available, tt.quantityErr).Once()
			pricingCalls := 0
			provider := priceProviderFunc(func(gotCtx context.Context, sku string, quantity int) (inventory.Price, error) {
				pricingCalls++
				assert.Equal(t, ctx, gotCtx)
				assert.Equal(t, "SKU-42", sku)
				assert.Equal(t, 2, quantity)
				return inventory.Price{UnitPrice: 12.5, Total: 25}, tt.priceErr
			})
			if tt.wantReserve {
				repository.EXPECT().Reserve(ctx, "SKU-42", 2).Run(func(context.Context, string, int) {
					assert.Equal(t, 1, pricingCalls)
				}).Return(tt.reserveErr).Once()
			}
			reservations := inventory.NewReservations(repository, provider)

			reservation, err := reservations.ReserveStock(ctx, "SKU-42", 2)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, reservation)
			} else {
				require.NoError(t, err)
				assert.Equal(t, inventory.Reservation{SKU: "SKU-42", Quantity: 2, UnitPrice: 12.5, Total: 25}, reservation)
			}
			if tt.wantPricing {
				assert.Equal(t, 1, pricingCalls)
			} else {
				assert.Zero(t, pricingCalls)
			}
			if !tt.wantReserve {
				repository.AssertNotCalled(t, "Reserve")
			}
		})
	}
}
