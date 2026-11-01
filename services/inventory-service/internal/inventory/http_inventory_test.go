package inventory_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory/gateway"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fixture struct {
	router     http.Handler
	repository *mocks.MockStockRepository
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

	repository := mocks.NewMockStockRepository(t)
	pricingClient := gateway.NewPricingHTTPClient(pricingServer.URL,
		otelhttp.WithTracerProvider(tracerProvider),
		otelhttp.WithPropagators(propagator),
	)
	reservations := inventory.NewReservations(repository, pricingClient)
	router := inventory.NewHttpRouter(reservations,
		otelgin.WithTracerProvider(tracerProvider),
		otelgin.WithMeterProvider(meterProvider),
		otelgin.WithPropagators(propagator),
	)
	return &fixture{router: router, repository: repository, spans: spans, metrics: reader, pricingHit: hit}
}

func (f *fixture) hasStock(sku string, quantity int) {
	f.repository.EXPECT().Quantity(mock.Anything, sku).Return(quantity, nil)
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
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)

	f.do(http.MethodGet, "/stock/SKU-42", "")

	server := f.spanByKind(t, oteltrace.SpanKindServer)
	assert.Contains(t, server.Attributes, attribute.String("http.route", "/stock/:sku"))
}

func TestServerDurationMetricCarriesRouteMethodAndStatus(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)

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
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)

	response := f.do(http.MethodGet, "/healthz", "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, f.spans.GetSpans())
}

func TestReservationPropagatesTraceContextToPricing(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)
	f.repository.EXPECT().Reserve(mock.Anything, "SKU-42", 2).Return(nil)

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
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)
	f.repository.EXPECT().Reserve(mock.Anything, "SKU-42", 2).Return(nil)

	f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":2}`)

	server := f.spanByKind(t, oteltrace.SpanKindServer)
	assert.Contains(t, server.Attributes, attribute.String("http.route", "/reservations"))
}

func TestReservationCallsPricingWithSkuAndQuantity(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)
	f.repository.EXPECT().Reserve(mock.Anything, "SKU-42", 2).Return(nil)

	f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":2}`)

	f.pricingHit.mu.Lock()
	defer f.pricingHit.mu.Unlock()
	assert.Equal(t, "/prices/SKU-42", f.pricingHit.path)
	assert.Equal(t, "quantity=2", f.pricingHit.query)
}

func TestReservationReturnsPriceAndDecrementsStock(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)
	f.repository.EXPECT().Reserve(mock.Anything, "SKU-42", 2).Return(nil).Once()

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":2}`)

	require.Equal(t, http.StatusCreated, response.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{"sku": "SKU-42", "quantity": 2.0, "unitPrice": 12.5, "total": 25.0}, body)
}

func TestReservationRejectedWithConflictWhenStockIsInsufficient(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":11}`)

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.JSONEq(t, `{"error":"insufficient stock"}`, response.Body.String())
}

func TestReservationRejectedWithNotFoundWhenSkuIsUnknownToInventory(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.repository.EXPECT().Quantity(mock.Anything, "SKU-NONE").Return(0, inventory.ErrSkuNotFound)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-NONE","quantity":1}`)

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.JSONEq(t, `{"error":"unknown sku"}`, response.Body.String())
}

func TestReservationRejectedWithNotFoundWhenPricingDoesNotKnowSku(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusNotFound, `{}`)
	f.hasStock("SKU-42", 10)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":1}`)

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.JSONEq(t, `{"error":"unknown sku"}`, response.Body.String())
}

func TestReservationFailsWithServerErrorWhenPricingIsDown(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusInternalServerError, `{}`)
	f.hasStock("SKU-42", 10)

	response := f.do(http.MethodPost, "/reservations", `{"sku":"SKU-42","quantity":1}`)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
}

func TestReservationRejectsInvalidBodies(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		body   string
		status int
	}{
		"malformed json":    {`{`, http.StatusBadRequest},
		"empty body":        {``, http.StatusBadRequest},
		"missing sku":       {`{"quantity":1}`, http.StatusUnprocessableEntity},
		"empty sku":         {`{"sku":"","quantity":1}`, http.StatusUnprocessableEntity},
		"missing quantity":  {`{"sku":"SKU-42"}`, http.StatusUnprocessableEntity},
		"wrong type":        {`{"sku":"SKU-42","quantity":"two"}`, http.StatusBadRequest},
		"zero quantity":     {`{"sku":"SKU-42","quantity":0}`, http.StatusUnprocessableEntity},
		"negative quantity": {`{"sku":"SKU-42","quantity":-3}`, http.StatusUnprocessableEntity},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, http.StatusOK, priceBody)

			response := f.do(http.MethodPost, "/reservations", tt.body)

			assert.Equal(t, tt.status, response.Code)
			assert.JSONEq(t, `{"error":"invalid reservation request"}`, response.Body.String())
		})
	}
}

func TestGetStockReturnsQuantity(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.hasStock("SKU-42", 10)

	response := f.do(http.MethodGet, "/stock/SKU-42", "")

	require.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"sku":"SKU-42","quantity":10}`, response.Body.String())
}

func TestGetStockReturnsNotFoundForUnknownSku(t *testing.T) {
	t.Parallel()

	f := newFixture(t, http.StatusOK, priceBody)
	f.repository.EXPECT().Quantity(mock.Anything, "SKU-NONE").Return(0, inventory.ErrSkuNotFound)

	response := f.do(http.MethodGet, "/stock/SKU-NONE", "")

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.JSONEq(t, `{"error":"unknown sku"}`, response.Body.String())
}

func TestInventoryLookupFailuresReturnGenericServerError(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, http.StatusOK, priceBody)
			f.repository.EXPECT().Quantity(mock.Anything, "SKU-42").Return(0, errors.New("private database details"))
			target := "/stock/SKU-42"
			if method == http.MethodPost {
				target = "/reservations"
			}

			response := f.do(method, target, `{"sku":"SKU-42","quantity":1}`)

			assert.Equal(t, http.StatusInternalServerError, response.Code)
			assert.Contains(t, response.Header().Get("Content-Type"), "application/json")
			assert.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
		})
	}
}
