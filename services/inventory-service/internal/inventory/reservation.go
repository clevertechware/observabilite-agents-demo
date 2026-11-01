package inventory

import (
	"context"
	"errors"
	"fmt"
)

type stockRepository interface {
	Quantity(ctx context.Context, sku string) (int, error)
	Reserve(ctx context.Context, sku string, quantity int) error
}

type priceProvider interface {
	Price(ctx context.Context, sku string, quantity int) (Price, error)
}

type Reservation struct {
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
	Total     float64 `json:"total"`
}

type Reservations struct {
	repository stockRepository
	provider   priceProvider
}

func NewReservations(repository stockRepository, provider priceProvider) *Reservations {
	return &Reservations{repository: repository, provider: provider}
}

// Quantity returns the quantity of the given SKU.
func (r *Reservations) Quantity(ctx context.Context, sku string) (int, error) {
	return r.repository.Quantity(ctx, sku)
}

// ReserveStock reserves the given quantity of the given SKU.
//
// It returns an error if the quantity is invalid or not available, or if there is an error retrieving the price.
func (r *Reservations) ReserveStock(ctx context.Context, sku string, quantity int) (Reservation, error) {
	if quantity <= 0 {
		return Reservation{}, ErrInvalidQuantity
	}

	available, err := r.repository.Quantity(ctx, sku)
	if err != nil {
		return Reservation{}, err
	}
	if available < quantity {
		return Reservation{}, ErrInsufficientStock
	}

	price, err := r.provider.Price(ctx, sku, quantity)
	if errors.Is(err, ErrPriceNotFound) {
		return Reservation{}, fmt.Errorf("%w: %w", ErrSkuNotFound, err)
	}
	if err != nil {
		return Reservation{}, err
	}
	if err = r.repository.Reserve(ctx, sku, quantity); err != nil {
		return Reservation{}, err
	}
	return Reservation{
		SKU:       sku,
		Quantity:  quantity,
		UnitPrice: price.UnitPrice,
		Total:     price.Total,
	}, nil
}
