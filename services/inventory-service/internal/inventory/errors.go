package inventory

import "errors"

var (
	// ErrSkuNotFound is returned when the SKU is not found.
	ErrSkuNotFound = errors.New("sku not found")
	// ErrInsufficientStock is returned when the quantity is not available.
	ErrInsufficientStock = errors.New("insufficient stock")
	// ErrPriceNotFound is returned when the price is not found.
	ErrPriceNotFound = errors.New("price not found")
)
