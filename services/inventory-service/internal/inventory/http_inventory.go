package inventory

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
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

type errorResponse struct {
	Error string `json:"error"`
}

func (h *httpHandlers) getStock(c *gin.Context) {
	ctx := c.Request.Context()
	sku := c.Param("sku")

	quantity, err := h.reservations.Quantity(ctx, sku)
	if errors.Is(err, ErrSkuNotFound) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "unknown sku"})
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "stock lookup failed", "sku", sku, "error", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}
	c.JSON(http.StatusOK, stockLevel{SKU: sku, Quantity: quantity})
}

func (h *httpHandlers) reserve(c *gin.Context) {
	ctx := c.Request.Context()
	var request reservationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		status := http.StatusBadRequest
		if _, ok := errors.AsType[validator.ValidationErrors](err); ok {
			status = http.StatusUnprocessableEntity
		}
		c.JSON(status, errorResponse{Error: "invalid reservation request"})
		return
	}

	reservation, err := h.reservations.ReserveStock(ctx, request.SKU, request.Quantity)
	switch {
	case errors.Is(err, ErrInvalidQuantity):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Error: "invalid reservation request"})
	case errors.Is(err, ErrInsufficientStock):
		slog.WarnContext(ctx, "insufficient stock", "sku", request.SKU, "quantity", request.Quantity)
		c.JSON(http.StatusConflict, errorResponse{Error: "insufficient stock"})
	case errors.Is(err, ErrSkuNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: "unknown sku"})
	case err != nil:
		slog.ErrorContext(ctx, "reservation failed", "sku", request.SKU, "error", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	default:
		slog.InfoContext(ctx, "stock reserved", "sku", request.SKU, "quantity", request.Quantity)
		c.JSON(http.StatusCreated, reservation)
	}
}
