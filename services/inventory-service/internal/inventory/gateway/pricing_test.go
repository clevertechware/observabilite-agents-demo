package gateway_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const pricingServiceResponse = `{"sku":"SKU-001","quantity":2,"unitPrice":19.99,"total":39.98}`

type remotePricing struct {
	server  *httptest.Server
	request *http.Request
}

func startRemotePricing(t *testing.T, status int, body string) *remotePricing {
	t.Helper()
	remote := &remotePricing{}
	remote.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote.request = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(remote.server.Close)
	return remote
}

func TestPriceCallsGetPricesWithSkuInPathAndQuantityInQuery(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusOK, pricingServiceResponse)

	_, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(context.Background(), "SKU-001", 2)

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, remote.request.Method)
	assert.Equal(t, "/prices/SKU-001", remote.request.URL.EscapedPath())
	assert.Equal(t, "quantity=2", remote.request.URL.RawQuery)
}

func TestPriceEscapesTheSkuInThePath(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusOK, pricingServiceResponse)

	_, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(context.Background(), "SKU/1 ?", 1)

	require.NoError(t, err)
	assert.Equal(t, "/prices/SKU%2F1%20%3F", remote.request.URL.EscapedPath())
}

func TestPriceReadsUnitPriceAndTotalFromTheCamelCaseResponse(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusOK, pricingServiceResponse)

	price, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(context.Background(), "SKU-001", 2)

	require.NoError(t, err)
	assert.Equal(t, inventory.Price{UnitPrice: 19.99, Total: 39.98}, price)
}

func TestPriceFailsWithPriceNotFoundWhenPricingAnswersNotFound(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusNotFound, `{"error":"unknown sku"}`)

	_, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(context.Background(), "SKU-999", 1)

	assert.ErrorIs(t, err, inventory.ErrPriceNotFound)
}

func TestPriceFailsWithTheStatusWhenPricingAnswersAnotherError(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			remote := startRemotePricing(t, status, `{}`)

			_, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(context.Background(), "SKU-001", 1)

			require.Error(t, err)
			assert.NotErrorIs(t, err, inventory.ErrPriceNotFound)
			assert.ErrorContains(t, err, "gateway returned status")
		})
	}
}

func TestPriceFailsWhenTheSuccessfulResponseIsNotJson(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusOK, `not json`)

	_, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(context.Background(), "SKU-001", 1)

	assert.ErrorContains(t, err, "decode price")
}

func TestPriceFailsWhenPricingIsUnreachable(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusOK, pricingServiceResponse)
	remote.server.Close()

	_, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(context.Background(), "SKU-001", 1)

	assert.Error(t, err)
}

func TestPriceFailsWhenTheContextIsCancelled(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusOK, pricingServiceResponse)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := gateway.NewPricingHTTPClient(remote.server.URL).Price(ctx, "SKU-001", 1)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPriceSendsTheTraceParentOfTheCurrentSpan(t *testing.T) {
	t.Parallel()

	remote := startRemotePricing(t, http.StatusOK, pricingServiceResponse)
	spans := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	propagator := propagation.TraceContext{}
	client := gateway.NewPricingHTTPClient(remote.server.URL,
		otelhttp.WithTracerProvider(tracerProvider),
		otelhttp.WithPropagators(propagator),
	)
	ctx, parent := tracerProvider.Tracer("test").Start(context.Background(), "parent")
	defer parent.End()

	_, err := client.Price(ctx, "SKU-001", 1)

	require.NoError(t, err)
	received := oteltrace.SpanContextFromContext(propagator.Extract(
		context.Background(), propagation.HeaderCarrier(remote.request.Header)))
	assert.Equal(t, parent.SpanContext().TraceID(), received.TraceID())
	require.Len(t, spans.GetSpans(), 1)
	assert.Equal(t, spans.GetSpans()[0].SpanContext.SpanID(), received.SpanID())
}
