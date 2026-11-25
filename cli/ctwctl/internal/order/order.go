package order

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrInvalidQuantity = errors.New("invalid quantity")

// Order is the order returned by order-service; Location is only set on creation.
type Order struct {
	ID        int64       `json:"id"`
	SKU       string      `json:"sku"`
	Quantity  int         `json:"quantity"`
	UnitPrice json.Number `json:"unitPrice"`
	Total     json.Number `json:"total"`
	Location  string      `json:"location,omitempty"`
}

type Client interface {
	Create(ctx context.Context, sku string, quantity int) (Order, error)
	Get(ctx context.Context, id int64) (Order, error)
}

type Orders struct {
	client Client
}

func NewOrders(client Client) *Orders {
	return &Orders{client: client}
}

func (o *Orders) Create(ctx context.Context, sku string, quantity int) (Order, error) {
	if quantity <= 0 {
		return Order{}, ErrInvalidQuantity
	}
	return o.client.Create(ctx, sku, quantity)
}

func (o *Orders) Get(ctx context.Context, id int64) (Order, error) {
	return o.client.Get(ctx, id)
}
