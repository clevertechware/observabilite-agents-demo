package shop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderClientCreate_WhenStockIsInsufficient_ReturnsTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/orders", r.URL.Path)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	_, err := NewOrderClient(server.URL).Create(t.Context(), "SKU-LOW", 5)

	assert.ErrorIs(t, err, ErrInsufficientStock)
}

func TestOrderClientCreate_WhenServerAnswers201_ReturnsOrderWithLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body createRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, createRequest{SKU: "SKU-001", Quantity: 2}, body)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Location", "/orders/42")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42,"sku":"SKU-001","quantity":2,"unitPrice":9.90,"total":19.80}`))
	}))
	defer server.Close()

	order, err := NewOrderClient(server.URL).Create(t.Context(), "SKU-001", 2)

	require.NoError(t, err)
	assert.Equal(t, Order{
		ID: 42, SKU: "SKU-001", Quantity: 2,
		UnitPrice: "9.90", Total: "19.80", Location: "/orders/42",
	}, order)
}

func TestOrderClientCreate_WhenServerAnswers200_ReportsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := NewOrderClient(server.URL).Create(t.Context(), "SKU-001", 1)

	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusOK, statusErr.Status)
}

func TestOrderClientErrors_MapStatusToTypedErrorWithProblemDetail(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   error
	}{
		{"invalid body is a 400", http.StatusBadRequest, ErrInvalidRequest},
		{"unknown sku is a 404", http.StatusNotFound, ErrNotFound},
		{"missing stock is a 409", http.StatusConflict, ErrInsufficientStock},
		{"inventory outage is a 502", http.StatusBadGateway, ErrUpstreamFailure},
		{"rate limiting is a 429", http.StatusTooManyRequests, ErrRateLimited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"detail":"détail du serveur"}`))
			}))
			defer server.Close()

			_, err := NewOrderClient(server.URL).Create(t.Context(), "SKU-001", 1)

			require.ErrorIs(t, err, tt.want)
			assert.Contains(t, err.Error(), "détail du serveur")
		})
	}
}

func TestOrderClientCreate_WhenIPIsNotAllowed_ReturnsStatusErrorWithoutSentinel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := NewOrderClient(server.URL).Create(t.Context(), "SKU-001", 1)

	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusForbidden, statusErr.Status)
}

func TestOrderClientGet_ReadsTheOrderAtItsPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/orders/42", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":42,"sku":"SKU-001","quantity":2,"unitPrice":9.90,"total":19.80}`))
	}))
	defer server.Close()

	order, err := NewOrderClient(server.URL).Get(t.Context(), 42)

	require.NoError(t, err)
	assert.EqualValues(t, 42, order.ID)
	assert.Empty(t, order.Location)
}

func TestOrderClientGet_WhenOrderIsMissing_ReturnsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := NewOrderClient(server.URL).Get(t.Context(), 7)

	assert.ErrorIs(t, err, ErrNotFound)
}
