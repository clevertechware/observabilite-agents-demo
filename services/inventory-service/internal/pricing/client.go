package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var ErrPriceNotFound = errors.New("price not found")

type Price struct {
	UnitPrice float64 `json:"unitPrice"`
	Total     float64 `json:"total"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, transportOptions ...otelhttp.Option) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport, transportOptions...),
			Timeout:   5 * time.Second,
		},
	}
}

func (c *Client) Price(ctx context.Context, sku string, quantity int) (Price, error) {
	target := fmt.Sprintf("%s/prices/%s?quantity=%d", c.baseURL, url.PathEscape(sku), quantity)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Price{}, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return Price{}, err
	}
	defer func() { _ = response.Body.Close() }()
	return decodePrice(response)
}

func decodePrice(response *http.Response) (Price, error) {
	switch response.StatusCode {
	case http.StatusOK:
		var price Price
		if err := json.NewDecoder(response.Body).Decode(&price); err != nil {
			return Price{}, fmt.Errorf("decode price: %w", err)
		}
		return price, nil
	case http.StatusNotFound:
		return Price{}, ErrPriceNotFound
	default:
		return Price{}, fmt.Errorf("pricing returned status %d", response.StatusCode)
	}
}
