package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type PricingHTTPClient struct {
	baseURL string
	http    *http.Client
}

func NewPricingHTTPClient(baseURL string, transportOptions ...otelhttp.Option) *PricingHTTPClient {
	return &PricingHTTPClient{
		baseURL: baseURL,
		http: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport, transportOptions...),
			Timeout:   5 * time.Second,
		},
	}
}

func (c *PricingHTTPClient) Price(ctx context.Context, sku string, quantity int) (inventory.Price, error) {
	target := fmt.Sprintf("%s/prices/%s?quantity=%d", c.baseURL, url.PathEscape(sku), quantity)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return inventory.Price{}, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return inventory.Price{}, err
	}
	defer func() { _ = response.Body.Close() }()
	return decodePrice(response)
}

func decodePrice(response *http.Response) (inventory.Price, error) {
	switch response.StatusCode {
	case http.StatusOK:
		var price inventory.Price
		if err := json.NewDecoder(response.Body).Decode(&price); err != nil {
			return inventory.Price{}, fmt.Errorf("decode price: %w", err)
		}
		return price, nil
	case http.StatusNotFound:
		return inventory.Price{}, inventory.ErrPriceNotFound
	default:
		return inventory.Price{}, fmt.Errorf("gateway returned status %d", response.StatusCode)
	}
}
