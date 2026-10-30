package store_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/store"
)

const postgresImage = "postgres:18.6-trixie"

var (
	spans          = tracetest.NewInMemoryExporter()
	tracerProvider = sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
)

// otelpgx records nothing unless the context already carries a recording span,
// which in the service is the server span created by otelgin.
func requestContext() context.Context {
	ctx, _ := tracerProvider.Tracer("test").Start(context.Background(), "request")
	return ctx
}

func seed(t *testing.T, pool *pgxpool.Pool, sku string, quantity int) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO stock (sku, quantity) VALUES ($1, $2)`, sku, quantity)
	require.NoError(t, err)
}

func queryTexts(sqlFragment string) []string {
	var texts []string
	for _, span := range spans.GetSpans() {
		for _, attr := range span.Attributes {
			value := attr.Value.String()
			if attr.Key == "db.query.text" && strings.Contains(value, sqlFragment) {
				texts = append(texts, value)
			}
		}
	}
	return texts
}

func TestQueryTextKeepsPlaceholderAndOmitsSku(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 10)

	_, err := store.NewRepository(pool, 0).Quantity(requestContext(), "SKU-42")

	require.NoError(t, err)
	texts := queryTexts("FROM stock")
	require.NotEmpty(t, texts)
	for _, text := range texts {
		assert.Contains(t, text, "$1")
		assert.NotContains(t, text, "SKU-42")
	}
}

func TestQuantityReturnsStoredQuantity(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 10)

	quantity, err := store.NewRepository(pool, 0).Quantity(requestContext(), "SKU-42")

	require.NoError(t, err)
	assert.Equal(t, 10, quantity)
}

func TestQuantityReportsUnknownSku(t *testing.T) {
	pool := newPool(t)

	_, err := store.NewRepository(pool, 0).Quantity(requestContext(), "SKU-NONE")

	assert.ErrorIs(t, err, store.ErrSkuNotFound)
}

func TestSlowQueryFaultDelaysReadByConfiguredSeconds(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 10)
	const delay = 700 * time.Millisecond

	started := time.Now()
	_, err := store.NewRepository(pool, delay.Seconds()).Quantity(requestContext(), "SKU-42")

	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(started), delay)
}

func TestSlowQueryFaultValueIsAbsentFromQueryText(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 10)

	_, err := store.NewRepository(pool, 0.25).Quantity(requestContext(), "SKU-42")

	require.NoError(t, err)
	texts := queryTexts("pg_sleep")
	require.NotEmpty(t, texts)
	for _, text := range texts {
		assert.Contains(t, text, "pg_sleep($2::float8)")
		assert.NotContains(t, text, "0.25")
	}
}

func TestQueryTextIsIdenticalWhateverTheFaultValue(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 10)

	_, err := store.NewRepository(pool, 0).Quantity(requestContext(), "SKU-42")
	require.NoError(t, err)
	_, err = store.NewRepository(pool, 0.1).Quantity(requestContext(), "SKU-42")
	require.NoError(t, err)

	distinct := map[string]struct{}{}
	for _, text := range queryTexts("pg_sleep") {
		distinct[text] = struct{}{}
	}
	assert.Len(t, distinct, 1)
}

func TestReserveDecrementsStock(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 10)
	repository := store.NewRepository(pool, 0)

	require.NoError(t, repository.Reserve(requestContext(), "SKU-42", 4))

	quantity, err := repository.Quantity(requestContext(), "SKU-42")
	require.NoError(t, err)
	assert.Equal(t, 6, quantity)
}

func TestReserveRefusesMoreThanAvailableAndKeepsStock(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 3)
	repository := store.NewRepository(pool, 0)

	err := repository.Reserve(requestContext(), "SKU-42", 4)

	assert.ErrorIs(t, err, store.ErrInsufficientStock)
	quantity, err := repository.Quantity(requestContext(), "SKU-42")
	require.NoError(t, err)
	assert.Equal(t, 3, quantity)
}

func TestReserveReportsUnknownSku(t *testing.T) {
	pool := newPool(t)

	err := store.NewRepository(pool, 0).Reserve(requestContext(), "SKU-NONE", 1)

	assert.ErrorIs(t, err, store.ErrSkuNotFound)
}

func TestConcurrentReservationsNeverOversellStock(t *testing.T) {
	pool := newPool(t)
	seed(t, pool, "SKU-42", 5)
	repository := store.NewRepository(pool, 0)

	const attempts = 20
	results := make(chan error, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- repository.Reserve(requestContext(), "SKU-42", 1)
		}()
	}
	group.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
			continue
		}
		assert.ErrorIs(t, err, store.ErrInsufficientStock)
	}
	assert.Equal(t, 5, succeeded)
	quantity, err := repository.Quantity(requestContext(), "SKU-42")
	require.NoError(t, err)
	assert.Equal(t, 0, quantity)
}
