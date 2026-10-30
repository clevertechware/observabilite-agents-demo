package store_test

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/store"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var databaseURL string

func TestMain(m *testing.M) {
	os.Exit(runWithPostgres(m))
}

func runWithPostgres(m *testing.M) int {
	ctx := context.Background()
	container, err := startPostgres(ctx)
	if err != nil {
		log.Printf("start postgres: %v", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			log.Printf("terminate postgres: %v", err)
		}
	}()

	databaseURL, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("postgres connection string: %v", err)
		return 1
	}
	return m.Run()
}

func startPostgres(ctx context.Context) (*postgres.PostgresContainer, error) {
	return postgres.Run(ctx, postgresImage,
		postgres.WithDatabase("inventory"),
		postgres.WithUsername("inventory"),
		postgres.WithPassword("inventory"),
		postgres.WithInitScripts(filepath.Join("..", "..", "schema.sql")),
		postgres.BasicWaitStrategies(),
	)
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	spans.Reset()
	pool, err := store.NewPool(context.Background(), databaseURL, otelpgx.WithTracerProvider(tracerProvider))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(context.Background(), `TRUNCATE stock`)
	require.NoError(t, err)
	return pool
}
