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
)

var (
	ErrInvalidRequest    = errors.New("requête refusée par order-service")
	ErrNotFound          = errors.New("SKU inconnu ou commande absente")
	ErrInsufficientStock = errors.New("stock insuffisant")
	ErrUpstreamFailure   = errors.New("inventory-service indisponible")
	ErrRateLimited       = errors.New("limite de débit atteinte")
)

// Order est la commande renvoyée par order-service ; Location n'est renseigné qu'à la création.
type Order struct {
	ID        int64       `json:"id"`
	SKU       string      `json:"sku"`
	Quantity  int         `json:"quantity"`
	UnitPrice json.Number `json:"unitPrice"`
	Total     json.Number `json:"total"`
	Location  string      `json:"location,omitempty"`
}

type StatusError struct {
	Status int
	Detail string
}

func (e *StatusError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("order-service a répondu %d", e.Status)
	}
	return fmt.Sprintf("order-service a répondu %d : %s", e.Status, e.Detail)
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

func (c *OrderClient) Create(ctx context.Context, sku string, quantity int) (Order, error) {
	payload, err := json.Marshal(createRequest{SKU: sku, Quantity: quantity})
	if err != nil {
		return Order{}, fmt.Errorf("encodage de la commande : %w", err)
	}
	resp, err := c.do(ctx, http.MethodPost, "/orders", payload)
	if err != nil {
		return Order{}, err
	}
	if resp.statusCode != http.StatusCreated {
		return Order{}, statusError(resp.statusCode, resp.body)
	}
	order, err := decodeOrder(resp.body)
	if err != nil {
		return Order{}, err
	}
	order.Location = resp.location
	return order, nil
}

func (c *OrderClient) Get(ctx context.Context, id int64) (Order, error) {
	resp, err := c.do(ctx, http.MethodGet, "/orders/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return Order{}, err
	}
	if resp.statusCode != http.StatusOK {
		return Order{}, statusError(resp.statusCode, resp.body)
	}
	return decodeOrder(resp.body)
}

func (c *OrderClient) do(ctx context.Context, method, path string, payload []byte) (response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return response{}, fmt.Errorf("construction de la requête : %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return response{}, fmt.Errorf("appel à order-service : %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response{}, fmt.Errorf("lecture de la réponse : %w", err)
	}
	return response{statusCode: resp.StatusCode, location: resp.Header.Get("Location"), body: body}, nil
}

func decodeOrder(body []byte) (Order, error) {
	var order Order
	if err := json.Unmarshal(body, &order); err != nil {
		return Order{}, fmt.Errorf("décodage de la commande : %w", err)
	}
	return order, nil
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
	return fmt.Errorf("%w : %s", sentinel, problem.Detail)
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
