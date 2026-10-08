package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/api"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/pricing"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fakeRepository struct {
	quantities map[string]int
	reserved   []string
}

func (f *fakeRepository) Quantity(_ context.Context, sku string) (int, error) {
	quantity, ok := f.quantities[sku]
	if !ok {
		return 0, store.ErrSkuNotFound
	}
	return quantity, nil
}

func (f *fakeRepository) Reserve(_ context.Context, sku string, _ int) error {
	f.reserved = append(f.reserved, sku)
	return nil
}

type fixture struct {
	router     http.Handler
	repository *fakeRepository
	spans      *tracetest.InMemoryExporter
	metrics    *sdkmetric.ManualReader
	pricingHit *recordedRequest
}

type recordedRequest struct {
	mu     sync.Mutex
	header http.Header
	path   string
	query  string
}

func (r *recordedRequest) record(request *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.header = request.Header.Clone()
	r.path = request.URL.Path
	r.query = request.URL.RawQuery
}

func newFixture(t *testing.T, pricingStatus int, pricingBody string) *fixture {
	t.Helper()
	hit := &recordedRequest{}
	pricingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.record(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(pricingStatus)
		_, _ = w.Write([]byte(pricingBody))
	}))
	t.Cleanup(pricingServer.Close)

	spans := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	propagator := propagation.TraceContext{}

	repository := &fakeRepository{quantities: map[string]int{"SKU-42": 10}}
	pricingClient := pricing.NewClient(pricingServer.URL,
		otelhttp.WithTracerProvider(tracerProvider),
		otelhttp.WithPropagators(propagator),
	)
	router := api.NewRouter(repository, pricingClient,
		otelgin.WithTracerProvider(tracerProvider),
		otelgin.WithMeterProvider(meterProvider),
		otelgin.WithPropagators(propagator),
	)
	return &fixture{router: router, repository: repository, spans: spans, metrics: reader, pricingHit: hit}
}

func (f *fixture) do(method, target, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	return recorder
}

func (f *fixture) spanByKind(t *testing.T, kind oteltrace.SpanKind) tracetest.SpanStub {
	t.Helper()
	for _, span := range f.spans.GetSpans() {
		if span.SpanKind == kind {
			return span
		}
	}
	require.Failf(t, "span not found", "no span of kind %s", kind)
	return tracetest.SpanStub{}
}

const priceBody = `{"unitPrice": 12.5, "total": 25}`

func TestServerSpanCarriesRouteTemplate(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	f.do(http.MethodGet, "/stock/SKU-42", "")

	server := f.spanByKind(t, oteltrace.SpanKindServer)
	assert.Contains(t, server.Attributes, attribute.String("http.route", "/stock/:sku"))
}

func TestServerDurationMetricCarriesRouteMethodAndStatus(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	f.do(http.MethodGet, "/stock/SKU-42", "")

	var collected metricdata.ResourceMetrics
	require.NoError(t, f.metrics.Collect(context.Background(), &collected))
	var duration *metricdata.Metrics
	for _, scope := range collected.ScopeMetrics {
		for i := range scope.Metrics {
			if scope.Metrics[i].Name == "http.server.request.duration" {
				duration = &scope.Metrics[i]
			}
		}
	}
	require.NotNil(t, duration)
	assert.Equal(t, "s", duration.Unit)
	histogram, ok := duration.Data.(metricdata.Histogram[float64])
	require.True(t, ok)
	require.Len(t, histogram.DataPoints, 1)
	attributes := histogram.DataPoints[0].Attributes
	assertAttribute(t, attributes, "http.route", "/stock/:sku")
	assertAttribute(t, attributes, "http.request.method", "GET")
	status, found := attributes.Value("http.response.status_code")
	require.True(t, found)
	assert.Equal(t, int64(http.StatusOK), status.AsInt64())
}

func assertAttribute(t *testing.T, set attribute.Set, key, want string) {
	t.Helper()
	value, found := set.Value(attribute.Key(key))
	require.Truef(t, found, "attribute %s missing", key)
	assert.Equal(t, want, value.AsString())
}

func TestHealthzIsNotTraced(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodGet, "/healthz", "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, f.spans.GetSpans())
}

func TestReservationPropagatesTraceContextToPricing(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":2}`)

	require.Equal(t, http.StatusCreated, response.Code)
	server := f.spanByKind(t, oteltrace.SpanKindServer)
	client := f.spanByKind(t, oteltrace.SpanKindClient)
	assert.Equal(t, server.SpanContext.TraceID(), client.SpanContext.TraceID())
	assert.Equal(t, server.SpanContext.SpanID(), client.Parent.SpanID())

	f.pricingHit.mu.Lock()
	defer f.pricingHit.mu.Unlock()
	extracted := oteltrace.SpanContextFromContext(propagation.TraceContext{}.Extract(
		context.Background(), propagation.HeaderCarrier(f.pricingHit.header)))
	assert.Equal(t, client.SpanContext.TraceID(), extracted.TraceID())
	assert.Equal(t, client.SpanContext.SpanID(), extracted.SpanID())
}

func TestReservationRoutesTemplateOnServerSpan(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":2}`)

	server := f.spanByKind(t, oteltrace.SpanKindServer)
	assert.Contains(t, server.Attributes, attribute.String("http.route", "/reservations"))
}

func TestReservationCallsPricingWithSkuAndQuantity(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":2}`)

	f.pricingHit.mu.Lock()
	defer f.pricingHit.mu.Unlock()
	assert.Equal(t, "/prices/SKU-42", f.pricingHit.path)
	assert.Equal(t, "quantity=2", f.pricingHit.query)
}

func TestReservationReturnsPriceAndDecrementsStock(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":2}`)

	require.Equal(t, http.StatusCreated, response.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{"sku": "SKU-42", "quantity": 2.0, "unitPrice": 12.5, "total": 25.0}, body)
	assert.Equal(t, []string{"SKU-42"}, f.repository.reserved)
}

func TestReservationRejectedWithConflictWhenStockIsInsufficient(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":11}`)

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Empty(t, f.repository.reserved)
}

func TestReservationRejectedWithNotFoundWhenSkuIsUnknownToInventory(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-NONE","quantity":1}`)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestReservationRejectedWithNotFoundWhenPricingDoesNotKnowSku(t *testing.T) {
	f := newFixture(t, http.StatusNotFound, `{}`)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":1}`)

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Empty(t, f.repository.reserved)
}

func TestReservationFailsWithServerErrorWhenPricingIsDown(t *testing.T) {
	f := newFixture(t, http.StatusInternalServerError, `{}`)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":1}`)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Empty(t, f.repository.reserved)
}

func TestReservationRejectsInvalidBodies(t *testing.T) {
	tests := map[string]string{
		"malformed json":    `{`,
		"missing sku":       `{"quantity":1}`,
		"zero quantity":     `{"sku":"SKU-42","quantity":0}`,
		"negative quantity": `{"sku":"SKU-42","quantity":-3}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, http.StatusOK, priceBody)

			response := f.do(http.MethodPost, "/reservations", body)

			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestGetStockReturnsQuantity(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodGet, "/stock/SKU-42", "")

	require.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"sku":"SKU-42","quantity":10}`, response.Body.String())
}

func TestGetStockReturnsNotFoundForUnknownSku(t *testing.T) {
	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodGet, "/stock/SKU-NONE", "")

	assert.Equal(t, http.StatusNotFound, response.Code)
}
