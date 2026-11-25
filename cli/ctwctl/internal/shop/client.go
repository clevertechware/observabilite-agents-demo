package shop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
)

var (
	ErrInvalidRequest    = errors.New("request rejected by order-service")
	ErrNotFound          = errors.New("unknown SKU or missing order")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrUpstreamFailure   = errors.New("inventory-service unavailable")
	ErrRateLimited       = errors.New("rate limit reached")
)

type StatusError struct {
	Status int
	Detail string
}

func (e *StatusError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("order-service responded %d", e.Status)
	}
	return fmt.Sprintf("order-service responded %d: %s", e.Status, e.Detail)
}

type OrderClient struct {
	baseURL string
	http    *http.Client
}

func NewOrderClient(baseURL string) *OrderClient {
	return &OrderClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

type createRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type response struct {
	statusCode int
	location   string
	body       []byte
}

func (c *OrderClient) Create(ctx context.Context, sku string, quantity int) (order.Order, error) {
	payload, err := json.Marshal(createRequest{SKU: sku, Quantity: quantity})
	if err != nil {
		return order.Order{}, fmt.Errorf("encode order: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/orders", payload)
	if err != nil {
		return order.Order{}, err
	}

	if resp.statusCode != http.StatusCreated {
		return order.Order{}, statusError(resp.statusCode, resp.body)
	}

	created, err := decodeOrder(resp.body)
	if err != nil {
		return order.Order{}, err
	}

	created.Location = resp.location
	return created, nil
}

func (c *OrderClient) Get(ctx context.Context, id int64) (order.Order, error) {
	resp, err := c.do(ctx, http.MethodGet, "/orders/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return order.Order{}, err
	}

	if resp.statusCode != http.StatusOK {
		return order.Order{}, statusError(resp.statusCode, resp.body)
	}

	return decodeOrder(resp.body)
}

func (c *OrderClient) do(ctx context.Context, method, path string, payload []byte) (response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return response{}, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return response{}, fmt.Errorf("call order-service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response{}, fmt.Errorf("read response: %w", err)
	}

	return response{statusCode: resp.StatusCode, location: resp.Header.Get("Location"), body: body}, nil
}

func decodeOrder(body []byte) (order.Order, error) {
	var decoded order.Order
	if err := json.Unmarshal(body, &decoded); err != nil {
		return order.Order{}, fmt.Errorf("decode order: %w", err)
	}
	return decoded, nil
}

type problemDetail struct {
	Detail string `json:"detail"`
}

func statusError(status int, body []byte) error {
	var problem problemDetail
	_ = json.Unmarshal(body, &problem)

	sentinel := sentinelFor(status)
	if sentinel == nil {
		return &StatusError{Status: status, Detail: problem.Detail}
	}
	if problem.Detail == "" {
		return sentinel
	}
	return fmt.Errorf("%w: %s", sentinel, problem.Detail)
}

func sentinelFor(status int) error {
	switch status {
	case http.StatusBadRequest:
		return ErrInvalidRequest
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrInsufficientStock
	case http.StatusBadGateway:
		return ErrUpstreamFailure
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return nil
	}
}
