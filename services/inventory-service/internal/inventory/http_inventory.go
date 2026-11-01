package inventory

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type httpHandlers struct {
	reservations *Reservations
}

type reservationRequest struct {
	SKU      string `json:"sku" binding:"required"`
	Quantity int    `json:"quantity" binding:"required,min=1"`
}

type stockLevel struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

func (h *httpHandlers) getStock(c *gin.Context) {
	ctx := c.Request.Context()
	sku := c.Param("sku")

	quantity, err := h.reservations.Quantity(ctx, sku)
	if errors.Is(err, ErrSkuNotFound) {
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

func (h *httpHandlers) reserve(c *gin.Context) {
	ctx := c.Request.Context()
	var request reservationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reservation, err := h.reservations.ReserveStock(ctx, request.SKU, request.Quantity)
	switch {
	case errors.Is(err, ErrInsufficientStock):
		slog.WarnContext(ctx, "insufficient stock", "sku", request.SKU, "quantity", request.Quantity)
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient stock"})
	case errors.Is(err, ErrSkuNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown sku"})
	case err != nil:
		slog.ErrorContext(ctx, "reservation failed", "sku", request.SKU, "error", err)
		c.Status(http.StatusInternalServerError)
	default:
		slog.InfoContext(ctx, "stock reserved", "sku", request.SKU, "quantity", request.Quantity)
		c.JSON(http.StatusCreated, reservation)
	}
}
