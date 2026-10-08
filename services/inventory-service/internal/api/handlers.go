package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/pricing"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/store"
)

type StockRepository interface {
	Quantity(ctx context.Context, sku string) (int, error)
	Reserve(ctx context.Context, sku string, quantity int) error
}

type PriceProvider interface {
	Price(ctx context.Context, sku string, quantity int) (pricing.Price, error)
}

type handlers struct {
	repository StockRepository
	pricing    PriceProvider
}

type reservationRequest struct {
	SKU      string `json:"sku" binding:"required"`
	Quantity int    `json:"quantity" binding:"required,min=1"`
}

type reservation struct {
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unitPrice"`
	Total     float64 `json:"total"`
}

type stockLevel struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

func (h *handlers) getStock(c *gin.Context) {
	ctx := c.Request.Context()
	sku := c.Param("sku")

	quantity, err := h.repository.Quantity(ctx, sku)
	if errors.Is(err, store.ErrSkuNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown sku"})
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "stock lookup failed", "sku", sku, "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, stockLevel{SKU: sku, Quantity: quantity})
}

func (h *handlers) reserve(c *gin.Context) {
	ctx := c.Request.Context()
	var request reservationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reservation, err := h.reserveStock(ctx, request)
	switch {
	case errors.Is(err, store.ErrInsufficientStock):
		slog.WarnContext(ctx, "insufficient stock", "sku", request.SKU, "quantity", request.Quantity)
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient stock"})
	case errors.Is(err, store.ErrSkuNotFound), errors.Is(err, pricing.ErrPriceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown sku"})
	case err != nil:
		slog.ErrorContext(ctx, "reservation failed", "sku", request.SKU, "error", err)
		c.Status(http.StatusInternalServerError)
	default:
		slog.InfoContext(ctx, "stock reserved", "sku", request.SKU, "quantity", request.Quantity)
		c.JSON(http.StatusCreated, reservation)
	}
}

func (h *handlers) reserveStock(ctx context.Context, request reservationRequest) (reservation, error) {
	available, err := h.repository.Quantity(ctx, request.SKU)
	if err != nil {
		return reservation{}, err
	}
	if available < request.Quantity {
		return reservation{}, store.ErrInsufficientStock
	}

	price, err := h.pricing.Price(ctx, request.SKU, request.Quantity)
	if err != nil {
		return reservation{}, err
	}
	if err := h.repository.Reserve(ctx, request.SKU, request.Quantity); err != nil {
		return reservation{}, err
	}
	return reservation{
		SKU:       request.SKU,
		Quantity:  request.Quantity,
		UnitPrice: price.UnitPrice,
		Total:     price.Total,
	}, nil
}
